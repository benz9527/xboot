package xlog

import (
	// "archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/safearchive/zip"
	"github.com/google/safeopen"
	"go.uber.org/multierr"

	"github.com/benz9527/xboot/lib/infra"
)

type fileSizeUnit uint64

const (
	B fileSizeUnit = 1 << (10 * iota)
	KB
	MB
	_maxSize = 1024 * MB
)

type fileAgeUnit int64

const (
	backupDateTimeFormat             = "2006_01_02T15_04_05.999999999_Z07_00"
	Second               fileAgeUnit = fileAgeUnit(time.Duration(1 * time.Second))
	Minute               fileAgeUnit = fileAgeUnit(time.Duration(1 * time.Minute))
	Hour                 fileAgeUnit = fileAgeUnit(time.Duration(1 * time.Hour))
	Day                  fileAgeUnit = fileAgeUnit(time.Duration(1 * time.Hour * 24))
	_maxFileAge                      = 2 * 7 * Day
)

var (
	fileSizeRegexp = regexp.MustCompile(`^(\d+)(([kK]|[mM])?[bB])$`)
	fileAgeRegexp  = regexp.MustCompile(`^(\d+)(s|[sS]ec|[mM]in|[hH](our[s]?)?|[dD](ay[s]?)?)$`)
)

func parseFileSize(size string) (uint64, error) {
	res := fileSizeRegexp.FindAllStringSubmatch(size, -1)
	if res == nil || len(res) <= 0 || len(res[0]) < 3 || res[0][0] != size {
		return 0, infra.NewErrorStack("invalid file size unit")
	}
	var unit fileSizeUnit
	switch strings.ToUpper(res[0][2]) {
	case "B":
		unit = B
	case "KB":
		unit = KB
	case "MB":
		unit = MB
	}
	_size, _ := strconv.ParseUint(res[0][1], 10, 64)
	return _size * uint64(unit), nil
}

func parseFileAge(age string) (time.Duration, error) {
	res := fileAgeRegexp.FindAllStringSubmatch(age, -1)
	if res == nil || len(res) <= 0 || len(res[0]) < 3 || res[0][0] != age {
		return 0, infra.NewErrorStack("invalid file age unit")
	}
	var unit fileAgeUnit
	switch strings.ToUpper(res[0][2]) {
	case "S", "SEC":
		unit = Second
	case "M", "MIN":
		unit = Minute
	case "H", "HOUR", "HOURS":
		unit = Hour
	case "D", "DAY", "DAYS":
		unit = Day
	}
	num, _ := strconv.ParseInt(res[0][1], 10, 64)
	_age := time.Duration(num) * time.Duration(unit)
	if _age >= time.Duration(_maxFileAge) {
		_age = time.Duration(_maxFileAge)
	}
	return _age, nil
}

var _ io.WriteCloser = (*rotateLog)(nil)

type rotateLog struct {
	ctx               context.Context
	filePath          string
	filename          string
	fileMaxSize       string
	fileMaxAge        string
	fileZipName       string
	maxSize           uint64
	wroteSize         uint64
	mkdirOnce         sync.Once
	currentFile       atomic.Pointer[os.File]
	fileWatcher       atomic.Pointer[fsnotify.Watcher]
	fileMaxBackups    int
	fileCompressBatch int
	fileCompressible  bool
}

func (log *rotateLog) Write(p []byte) (n int, err error) {
	select {
	case <-log.ctx.Done():
		return 0, io.EOF
	default:
	}

	if log.currentFile.Load() == nil {
		if err := log.openOrCreate(); err != nil {
			return 0, err
		}
	}
	logLen := uint64(len(p))
	if log.wroteSize+logLen > log.maxSize {
		n, err = log.currentFile.Load().Write(p)
		if err != nil {
			return
		}
		if err = log.backupThenCreate(); err != nil {
			return
		}
		return
	}

	n, err = log.currentFile.Load().Write(p)
	log.wroteSize += uint64(n)
	return
}

func (log *rotateLog) Close() error {
	if log.currentFile.Load() == nil {
		return nil
	}
	if err := log.currentFile.Load().Close(); err != nil {
		return err
	}
	log.currentFile.Store(nil)
	return nil
}

func (log *rotateLog) initialize() error {
	if log.fileWatcher.Load() != nil {
		return nil
	}

	size, err := parseFileSize(log.fileMaxSize)
	if err != nil {
		handleRollingError(err)
		return err
	}
	log.maxSize = size

	if _, err = parseFileAge(log.fileMaxAge); err != nil {
		handleRollingError(err)
		return err
	}

	var watcher *fsnotify.Watcher
	if watcher, err = fsnotify.NewWatcher(); err != nil {
		handleRollingError(infra.WrapErrorStackWithMessage(err, "failed to create file watcher"))
		return err
	}
	log.fileWatcher.Store(watcher)

	if err = log.fileWatcher.Load().Add(log.filePath); err != nil {
		handleRollingError(infra.WrapErrorStackWithMessage(err, "failed to add log directory to watcher"))
		return err
	}

	go log.watchAndArchive()
	return nil
}

func (log *rotateLog) mkdir() error {
	var err error = nil
	log.mkdirOnce.Do(func() {
		if log.filePath == "" {
			log.filePath = os.TempDir()
		}
		if log.filePath == os.TempDir() {
			return
		}
		err = os.MkdirAll(log.filePath, 0o644)
	})
	return infra.WrapErrorStack(err)
}

func (log *rotateLog) backup() error {
	logName := log.filename
	ext := filepath.Ext(logName)
	logNamePrefix := strings.TrimSuffix(logName, ext)
	now := time.Now().UTC()
	ts := now.Format(backupDateTimeFormat)
	pathToBackup := filepath.Join(log.filePath, logNamePrefix+"_"+ts+ext)
	if err := log.currentFile.Load().Close(); err != nil {
		return infra.WrapErrorStackWithMessage(err, "failed to backup current log: "+filepath.Join(log.filePath, logName))
	}
	return os.Rename(filepath.Join(log.filePath, logName), pathToBackup)
}

func (log *rotateLog) create() error {
	if err := log.mkdir(); err != nil {
		return err
	}

	f, err := safeopen.OpenFileBeneath(log.filePath, log.filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return infra.WrapErrorStackWithMessage(err, "unable to create new log file: "+filepath.Join(log.filePath, log.filename))
	}
	log.currentFile.Store(f)
	log.wroteSize = 0
	return nil
}

func (log *rotateLog) backupThenCreate() error {
	if err := log.backup(); err != nil {
		return err
	}
	return log.create()
}

func (log *rotateLog) openOrCreate() error {
	if err := log.mkdir(); err != nil {
		return err
	}

	pathToLog := filepath.Join(log.filePath, log.filename)
	info, err := os.Stat(pathToLog)
	if os.IsNotExist(err) {
		var merr error
		merr = multierr.Append(merr, err)
		if err = log.create(); err != nil {
			return multierr.Append(merr, err)
		}
		return log.initialize()
	} else if err != nil {
		log.currentFile.Store(nil)
		return infra.WrapErrorStack(err)
	}

	if info.IsDir() {
		log.currentFile.Store(nil)
		return infra.NewErrorStack("log file <" + pathToLog + "> is a dir")
	}

	var f *os.File
	if f, err = safeopen.OpenFileBeneath(log.filePath, log.filename, os.O_WRONLY|os.O_APPEND, 0o644); err != nil {
		var merr error = infra.WrapErrorStackWithMessage(err, "unable to access log file: "+pathToLog)
		if err = log.backupThenCreate(); err != nil {
			return infra.WrapErrorStackWithMessage(multierr.Combine(merr, err), "failed to backup then open new log file: "+pathToLog)
		}
	}
	log.currentFile.Store(f)
	log.wroteSize = uint64(info.Size())
	return log.initialize()
}

// Watch the log directory and filter the match log files then archive
// or delete the expired. Endless until the rotate log is closed.
func (log *rotateLog) watchAndArchive() {
	ext := filepath.Ext(log.filename)
	logName := log.filename[:len(log.filename)-len(ext)]
	duration, _ := parseFileAge(log.fileMaxAge)
	for {
		select {
		case <-log.ctx.Done():
			_ = log.Close()
			handleRollingError(log.fileWatcher.Load().Close())
			log.fileWatcher.Store(nil)
			return
		case event, ok := <-log.fileWatcher.Load().Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) {
				// Walk through the log files and find the expired ones.
				logInfos, err := log.loadFileInfos(logName, ext)
				if err != nil || len(logInfos) <= 0 {
					handleRollingError(err)
					continue
				}
				now := time.Now().UTC()
				expired, rest := filterExpiredLogs(now, logName, ext, duration, logInfos)
				expired = filterMaxBackupLogs(expired, rest, log.fileMaxBackups)
				if log.fileCompressible {
					if len(expired) < log.fileCompressBatch {
						continue
					}
					if err := compressExpiredLogs(log.filePath, log.fileZipName, expired); err != nil {
						handleRollingError(err)
						continue
					}
				} else {
					for _, info := range expired {
						filename := filepath.Base(info.Name())
						_ = os.Remove(filepath.Join(log.filePath, filename))
					}
				}
			}
		case err, ok := <-log.fileWatcher.Load().Errors:
			if !ok {
				return
			}
			handleRollingError(err)
		}
	}
}

func (log *rotateLog) loadFileInfos(logName, ext string) ([]fs.FileInfo, error) {
	// Walk through the log files and find the expired ones.
	entries, err := os.ReadDir(log.filePath)
	if err == nil && len(entries) > 0 {
		logInfos := make([]os.FileInfo, 0, 16)
		for _, entry := range entries {
			if !entry.IsDir() {
				filename := entry.Name()
				if strings.HasPrefix(filename, logName) && strings.HasSuffix(filename, ext) && filename != log.filename {
					if info, err := entry.Info(); err == nil && info != nil {
						logInfos = append(logInfos, info)
					}
				}
			}
		}
		return logInfos, nil
	}
	return nil, infra.WrapErrorStack(err)
}

func RotateLog(ctx context.Context, cfg *FileCoreConfig) io.WriteCloser {
	if cfg == nil || ctx == nil {
		return nil
	}
	w := &rotateLog{
		filename:          cfg.Filename,
		filePath:          cfg.FilePath,
		fileCompressible:  cfg.FileCompressible,
		fileCompressBatch: cfg.FileCompressBatch,
		fileMaxAge:        cfg.FileMaxAge,
		fileZipName:       cfg.FileZipName,
		fileMaxSize:       cfg.FileMaxSize,
		fileMaxBackups:    cfg.FileMaxBackups,
		ctx:               ctx,
	}
	if err := w.initialize(); err != nil {
		return nil
	}
	return w
}

func filterExpiredLogs(now time.Time, logName, ext string, duration time.Duration, logInfos []fs.FileInfo) ([]fs.FileInfo, []fs.FileInfo) {
	// Firstly, we satisfy the max age requirement.
	expired := make([]os.FileInfo, 0, 16)
	rest := make([]os.FileInfo, 0, 16)
	for _, info := range logInfos {
		filename := filepath.Base(info.Name())
		if !strings.HasPrefix(filename, logName) || !strings.HasSuffix(filename, ext) {
			continue
		}
		ts := strings.TrimPrefix(filename, logName+"_")
		ts = strings.TrimSuffix(ts, ext)
		if dateTime, err := time.Parse(backupDateTimeFormat, ts); err == nil {
			if now.Sub(dateTime) > duration {
				expired = append(expired, info)
			} else {
				rest = append(rest, info)
			}
		}
	}
	return expired, rest
}

func handleRollingError(err error) {
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "[XLogger] rolling file occurs error: %s\n", err)
	}
}

func filterMaxBackupLogs(expired, rest []fs.FileInfo, maxBackups int) []fs.FileInfo {
	// Secondly, we satisfy the max backups requirement.
	redundant := len(rest) - maxBackups
	if redundant > 0 {
		sort.Slice(rest, func(i, j int) bool {
			// If the log file is modified manually, the sort maybe wrong!
			return rest[i].ModTime().Before(rest[j].ModTime())
		})
		for i := 0; i < redundant; i++ {
			expired = append(expired, rest[i])
		}
	}
	return expired
}

// Only one zip file will be presented.
func compressExpiredLogs(filePath, zipName string, expired []fs.FileInfo) error {
	var (
		logZip  *os.File
		prevZip *zip.ReadCloser
	)
	info, err := os.Stat(filepath.Join(filePath, zipName))
	if err == nil && !info.IsDir() {
		// Exists
		if logZip, err = safeopen.OpenFileBeneath(filePath, "xlog-tmp.zip", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644); err != nil {
			return err
		}
		if prevZip, err = zip.OpenReader(filepath.Join(filePath, zipName)); err != nil {
			return err
		}
	} else {
		if logZip, err = os.Create(filepath.Join(filePath, zipName)); err != nil {
			return err
		}
	}
	zipWriter := zip.NewWriter(logZip)
	for _, info := range expired {
		filename := filepath.Base(info.Name())
		file, err := safeopen.OpenBeneath(filePath, filename)
		if err == nil {
			if zipFile, err := zipWriter.Create(filename); err == nil {
				if _, err = io.Copy(zipFile, file); err == nil {
					_ = file.Close()
					file = nil
					if err = os.Remove(filepath.Join(filePath, filename)); err != nil {
						handleRollingError(err)
					}
				}
			}
			if file != nil {
				_ = file.Close()
			}
		}
	}
	// Copy previous zip content to new zip file.
	if prevZip != nil {
		prevZip.SetSecurityMode(prevZip.GetSecurityMode() | zip.MaximumSecurityMode)
		for _, f := range prevZip.File {
			oldReader, err := f.Open()
			if err != nil || f.Mode().IsDir() {
				if oldReader != nil {
					_ = oldReader.Close()
				}
				continue
			}

			header := &zip.FileHeader{
				Name:   f.Name,
				Method: f.Method,
			}
			if zipFile, err := zipWriter.CreateHeader(header); err == nil {
				if _, err = io.Copy(zipFile, oldReader); err == nil {
					_ = oldReader.Close()
				}
			}
			if oldReader != nil {
				_ = oldReader.Close()
			}
		}
		if err := zipWriter.Flush(); err != nil {
			return err
		}
	}
	_ = zipWriter.Close()
	zipWriter = nil
	_ = logZip.Close()
	if prevZip != nil {
		_ = prevZip.Close()
		if err = os.Remove(filepath.Join(filePath, zipName)); err != nil {
			handleRollingError(err)
		}
		if err := os.Rename(filepath.Join(filePath, "xlog-tmp.zip"), filepath.Join(filePath, zipName)); err != nil {
			handleRollingError(err)
		}
	}
	return nil
}
