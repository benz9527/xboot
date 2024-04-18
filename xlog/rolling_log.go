package xlog

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/benz9527/xboot/lib/infra"
)

var _ io.WriteCloser = (*RollingLog)(nil)

type fileSizeUnit uint64

const (
	B  fileSizeUnit = 1
	KB fileSizeUnit = 1 << (10 * iota)
	MB
	_maxUnit = MB
)

var fileSizeRegexp = regexp.MustCompile(`^(\d+)(([kK]|[mM])?[bB])$`)

func parseFileSizeUnit(size string) (uint64, error) {
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

type RollingLog struct {
	FilePath         string `json:"filePath" yaml:"filePath"`
	Filename         string `json:"filename" yaml:"filename"`
	FileMaxSize      string `json:"fileMaxSize" yaml:"fileMaxSize"`
	FileMaxBackups   int    `json:"fileMaxBackups" yaml:"fileMaxBackups"`
	FileMaxAge       int    `json:"fileMaxAge" yaml:"fileMaxAge"`
	maxSize          uint64
	wroteSize        uint64
	mkdirOnce        sync.Once
	currentFile      *os.File
	FileCompressible bool `json:"fileCompressible" yaml:"fileCompressible"`
}

func (log *RollingLog) Write(p []byte) (n int, err error) {
	if log.currentFile == nil {
		if err := log.createOrOpenLog(); err != nil {
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

func (log *RollingLog) filename() string {
	filename := log.Filename
	if filename == "" {
		filename = filepath.Base(os.Args[0]) + "_xboot.log"
	}
	return filename
}

func (log *RollingLog) backup() error {
	logName := log.filename()
	ext := filepath.Ext(logName)
	logNamePrefix := strings.TrimSuffix(logName, ext)
	now := time.Now().UTC()
	t := now.Format("2006_01_02T15_04_05Z07_00")
	pathToBackup := filepath.Join(log.FilePath, logNamePrefix+"_"+t+"."+ext)
	if err := log.currentFile.Close(); err != nil {
		return infra.WrapErrorStackWithMessage(err, "failed to backup current log: "+filepath.Join(log.FilePath, logName))
	}
	return os.Rename(log.Filename, pathToBackup)
}

func (log *RollingLog) create() error {
	if err := log.mkdir(); err != nil {
		return err
	}
	pathToLog := filepath.Join(log.FilePath, log.filename())
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

func (log *RollingLog) createOrOpenLog() error {
	if err := log.mkdir(); err != nil {
		return err
	}

	pathToLog := filepath.Join(log.FilePath, log.filename())
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

func NewRollingLogFromYaml() {
}
