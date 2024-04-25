package xlog

import (
	"io"
	"os"
	"path/filepath"
	"sync"

	"go.uber.org/multierr"

	"github.com/benz9527/xboot/lib/infra"
)

var _ io.WriteCloser = (*singleLog)(nil)

// singleLog is not thread-safe.
type singleLog struct {
	filePath    string
	filename    string
	wroteSize   uint64
	mkdirOnce   sync.Once
	currentFile *os.File
}

func (log *singleLog) Write(p []byte) (n int, err error) {
	if log.currentFile == nil {
		if err := log.openOrCreate(); err != nil {
			return 0, err
		}
	}
	n, err = log.currentFile.Write(p)
	log.wroteSize += uint64(n)
	return
}

func (log *singleLog) Close() error {
	if log.currentFile == nil {
		return nil
	}
	if err := log.currentFile.Close(); err != nil {
		return err
	}
	log.currentFile = nil
	return nil
}

func (log *singleLog) openOrCreate() error {
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
		return nil
	} else if err != nil {
		log.currentFile = nil
		return infra.WrapErrorStack(err)
	}

	if info.IsDir() {
		log.currentFile = nil
		return infra.NewErrorStack("log file <" + pathToLog + "> is a dir")
	}

	var f *os.File
	if f, err = os.OpenFile(pathToLog, os.O_WRONLY|os.O_APPEND, 0o644); err != nil {
		return infra.WrapErrorStackWithMessage(err, "failed to open an exists log file")
	}
	log.currentFile, log.wroteSize = f, uint64(info.Size())
	return nil
}

func (log *singleLog) mkdir() error {
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

func (log *singleLog) create() error {
	if err := log.mkdir(); err != nil {
		return err
	}
	pathToLog := filepath.Join(log.filePath, log.filename)
	f, err := os.OpenFile(pathToLog, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return infra.WrapErrorStackWithMessage(err, "unable to create new log file: "+pathToLog)
	}
	log.currentFile, log.wroteSize = f, 0
	return nil
}
