package xlog

import (
	"io"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/multierr"

	"github.com/benz9527/xboot/lib/infra"
)

var _ io.WriteCloser = (*SingleLog)(nil)

type SingleLog struct {
	FilePath    string
	Filename    string
	wroteSize   uint64
	mkdirOnce   sync.Once
	currentFile *os.File
}

func (log *SingleLog) Write(p []byte) (n int, err error) {
	if log.currentFile == nil {
		if err := log.openOrCreate(); err != nil {
			return 0, err
		}
	}
	n, err = log.currentFile.Write(p)
	log.wroteSize += uint64(n)
	return
}

func (log *SingleLog) Close() error {
	return log.currentFile.Close()
}

func (log *SingleLog) openOrCreate() error {
	if err := log.mkdir(); err != nil {
		return err
	}

	pathToLog := filepath.Join(log.FilePath, log.Filename)
	info, err := os.Stat(pathToLog)
	if os.IsNotExist(err) {
		var merr error
		merr = multierr.Append(merr, err)
		if err = log.create(); err != nil {
			return multierr.Append(merr, err)
		}
	} else if err == nil && !info.IsDir() {
		var f *os.File
		if f, err = os.OpenFile(pathToLog, os.O_WRONLY|os.O_APPEND, 0o644); err != nil {
			return infra.WrapErrorStackWithMessage(err, "failed to open an exists log file")
		}
		log.currentFile, log.wroteSize = f, uint64(info.Size())
	} else if err == nil && info.IsDir() {
		log.currentFile = nil
		return infra.NewErrorStack("log file <" + pathToLog + "> is a dir")
	} else if err != nil {
		log.currentFile = nil
		return infra.WrapErrorStack(err)
	}
	return nil
}

func (log *SingleLog) mkdir() error {
	var err error = nil
	log.mkdirOnce.Do(func() {
		if log.FilePath == "" {
			log.FilePath = os.TempDir()
			return
		}
		err = os.MkdirAll(log.FilePath, 0o644)
	})
	return infra.WrapErrorStack(err)
}

func (log *SingleLog) create() error {
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
