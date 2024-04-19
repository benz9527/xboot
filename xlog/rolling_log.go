package xlog

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/benz9527/xboot/lib/infra"
)

var _ io.WriteCloser = (*RollingLog)(nil)

type fileSizeUnit uint64

const (
	B  fileSizeUnit = 1
	KB fileSizeUnit = 1 << (10 * iota)
	MB
	_maxSize = 1024 * MB
)

type fileAgeUnit int64

const (
	backupDateTimeFormat             = "2006_01_02T15_04_05.999999999_Z07_00"
	S                    fileAgeUnit = fileAgeUnit(time.Duration(1 * time.Second))
	M                    fileAgeUnit = fileAgeUnit(time.Duration(1 * time.Minute))
	H                    fileAgeUnit = fileAgeUnit(time.Duration(1 * time.Hour))
	D                    fileAgeUnit = fileAgeUnit(time.Duration(1 * time.Hour * 24))
	_maxAge                          = 2 * 7 * D
)

var (
	fileSizeRegexp = regexp.MustCompile(`^(\d+)(([kK]|[mM])?[bB])$`)
	fileAgeRegexp  = regexp.MustCompile(`^(\d+)(s(ec)?|m(in)?|h(our)?|H|d(ay)|D)$`)
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
	_size, err := strconv.ParseUint(res[0][1], 10, 64)
	if err != nil {
		return 0, infra.WrapErrorStackWithMessage(err, "unknown file size")
	}
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
		unit = S
	case "M", "MIN":
		unit = M
	case "H", "HOUR":
		unit = H
	case "D", "DAY":
		unit = D
	}
	num, err := strconv.ParseInt(res[0][1], 10, 64)
	if err != nil {
		return 0, infra.WrapErrorStackWithMessage(err, "unknown file age")
	}
	return time.Duration(unit) * time.Duration(num), nil
}

type RollingLog struct {
	FilePath          string `json:"filePath" yaml:"filePath"`
	Filename          string `json:"filename" yaml:"filename"`
	FileMaxSize       string `json:"fileMaxSize" yaml:"fileMaxSize"`
	FileMaxAge        string `json:"fileMaxAge" yaml:"fileMaxAge"`
	maxSize           uint64
	wroteSize         uint64
	mkdirOnce         sync.Once
	currentFile       *os.File
	fileWatcher       *fsnotify.Watcher
	FileMaxBackups    int  `json:"fileMaxBackups" yaml:"fileMaxBackups"`
	FileCompressBatch int  `json:"fileCompressBatch" yaml:"fileCompressBatch"`
	FileCompressible  bool `json:"fileCompressible" yaml:"fileCompressible"`
}

func (log *RollingLog) Write(p []byte) (n int, err error) {
	if log.currentFile == nil {
		if err := log.openOrCreate(); err != nil {
			return 0, err
		}
	}
	logLen := uint64(len(p))
	if log.wroteSize+logLen > log.maxSize {
		n, err = log.currentFile.Write(p)
		if err != nil {
			return
		}
		if err = log.backupThenCreate(); err != nil {
			return
		}
		return
	}

	n, err = log.currentFile.Write(p)
	log.wroteSize += uint64(n)
	return
}

func (log *RollingLog) Close() error {
	return log.currentFile.Close()
}

func (log *RollingLog) mkdir() error {
	var err error
	log.mkdirOnce.Do(func() {
		if log.FilePath == "" {
			log.FilePath = os.TempDir()
		}
		err = os.MkdirAll(log.FilePath, 0o755)
	})
	return infra.WrapErrorStack(err)
}

func (log *RollingLog) backup() error {
	logName := log.Filename
	ext := filepath.Ext(logName)
	logNamePrefix := strings.TrimSuffix(logName, ext)
	now := time.Now().UTC()
	ts := now.Format(backupDateTimeFormat)
	pathToBackup := filepath.Join(log.FilePath, logNamePrefix+"_"+ts+ext)
	if err := log.currentFile.Close(); err != nil {
		return infra.WrapErrorStackWithMessage(err, "failed to backup current log: "+filepath.Join(log.FilePath, logName))
	}
	return os.Rename(filepath.Join(log.FilePath, logName), pathToBackup)
}

func (log *RollingLog) create() error {
	if err := log.mkdir(); err != nil {
		return err
	}
	pathToLog := filepath.Join(log.FilePath, log.Filename)
	f, err := os.OpenFile(pathToLog, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return infra.WrapErrorStackWithMessage(err, "unable to create new log file: "+pathToLog)
	}
	log.currentFile, log.wroteSize = f, 0
	return nil
}

func (log *RollingLog) backupThenCreate() error {
	if err := log.backup(); err != nil {
		return err
	}
	return log.create()
}

func (log *RollingLog) openOrCreate() error {
	if err := log.mkdir(); err != nil {
		return err
	}

	pathToLog := filepath.Join(log.FilePath, log.Filename)
	info, err := os.Stat(pathToLog)
	if os.IsNotExist(err) {
		if _err := log.create(); _err != nil {
			return _err
		}
	} else if err == nil && !info.IsDir() {
		f, _err := os.OpenFile(pathToLog, os.O_WRONLY|os.O_APPEND, 0o644)
		if _err != nil {
			if _err = log.backupThenCreate(); _err != nil {
				return infra.WrapErrorStackWithMessage(_err, "failed to open an exists log file")
			}
		}
		log.currentFile, log.wroteSize = f, uint64(info.Size())
	} else if err == nil && info.IsDir() {
		log.currentFile = nil
		return infra.NewErrorStack("log file: " + pathToLog + " is a dir")
	} else if err != nil {
		log.currentFile = nil
		return infra.WrapErrorStack(err)
	}
	return nil
}

func (log *RollingLog) watchAndArchive() {
	ext := filepath.Ext(log.Filename)
	logName := log.Filename[:len(log.Filename)-len(ext)]
	duration, _ := parseFileAge(log.FileMaxAge)
	for {
		select {
		case event, ok := <-log.fileWatcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) {
				// Walk through the log files and find the expired ones.
				entries, err := os.ReadDir(log.FilePath)
				if err == nil && len(entries) > 0 {
					logInfos := make([]os.FileInfo, 0, 16)
					for _, entry := range entries {
						if !entry.IsDir() {
							filename := entry.Name()
							if strings.HasPrefix(filename, logName) && strings.HasSuffix(filename, ext) && filename != log.Filename {
								if info, err := entry.Info(); err == nil && info != nil {
									logInfos = append(logInfos, info)
								}
							}
						}
					}

					// Firstly, we satisfy the max age requirement.
					expired := make([]os.FileInfo, 0, 16)
					rest := make([]os.FileInfo, 0, 16)
					now := time.Now().UTC()
					for _, info := range logInfos {
						filename := filepath.Base(info.Name())
						ts := strings.TrimPrefix(filename, logName+"_")
						ts = strings.TrimSuffix(ts, ext)
						dateTime, err := time.Parse(backupDateTimeFormat, ts)
						if err == nil {
							if now.Sub(dateTime) > duration {
								expired = append(expired, info)
							} else {
								rest = append(rest, info)
							}
						}
					}
					// Secondly, we satisfy the max backups requirement.
					redundant := len(rest) - log.FileMaxBackups
					if redundant > 0 {
						sort.Slice(rest, func(i, j int) bool {
							// If the log file is modified manually, the sort maybe wrong!
							return rest[i].ModTime().Before(rest[j].ModTime())
						})
						for i := 0; i < redundant; i++ {
							expired = append(expired, rest[i])
						}
					}

					if log.FileCompressible {
						if len(expired) < log.FileCompressBatch {
							continue
						}
						logZip, err := os.Create(filepath.Join(log.FilePath, logName+"_"+now.Format(backupDateTimeFormat)+".zip"))
						if err != nil {
							continue
						}
						writer := zip.NewWriter(logZip)
						for _, info := range expired {
							filename := filepath.Base(info.Name())
							file, err := os.Open(filepath.Join(log.FilePath, filename))
							if err == nil {
								if zipFile, err := writer.Create(filename); err == nil {
									if _, err = io.Copy(zipFile, file); err == nil {
										_ = file.Close()
										file = nil
										if err = os.Remove(filepath.Join(log.FilePath, info.Name())); err != nil {
											panic(err)
										}
									}
								}
								if file != nil {
									_ = file.Close()
								}
							}
						}
						_ = writer.Close()
						_ = logZip.Close()
					} else {
						for _, info := range expired {
							filename := filepath.Base(info.Name())
							_ = os.Remove(filepath.Join(log.FilePath, filename))
						}
					}
				}
			}
		case err, ok := <-log.fileWatcher.Errors:
			if !ok {
				return
			}
			panic(err) // TODO we have to handle the fsnotify error
		}
	}
}

func Init() {

}
