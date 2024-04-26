package xlog

import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

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
	currentFile atomic.Pointer[os.File]
	closeC      <-chan struct{}
}

func (log *singleLog) Write(p []byte) (n int, err error) {
	select {
	case <-log.closeC:
		return 0, io.EOF
	default:
	}

	if log.currentFile.Load() == nil {
		if err := log.openOrCreate(); err != nil {
			return 0, err
		}
	}
	n, err = log.currentFile.Load().Write(p)
	log.wroteSize += uint64(n)
	return
}

func (log *singleLog) Close() error {
	if log.currentFile.Load() == nil {
		return nil
	}
	if err := log.currentFile.Load().Close(); err != nil {
		return err
	}
	log.currentFile.Store(nil)
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
		log.currentFile.Store(nil)
		return infra.WrapErrorStack(err)
	}

	if info.IsDir() {
		log.currentFile.Store(nil)
		return infra.NewErrorStack("log file <" + pathToLog + "> is a dir")
	}

	var f *os.File
	if f, err = os.OpenFile(pathToLog, os.O_WRONLY|os.O_APPEND, 0o644); err != nil {
		return infra.WrapErrorStackWithMessage(err, "failed to open an exists log file")
	}
	log.currentFile.Store(f)
	log.wroteSize = uint64(info.Size())
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
	log.currentFile.Store(f)
	log.wroteSize = 0
	return nil
}

func (log *singleLog) initialize() error {
	go func() {
		select {
		case <-log.closeC:
			_ = log.Close()
			return
		}
	}()
	return nil
}

func SingleLog(cfg *FileCoreConfig, closeC chan struct{}) io.WriteCloser {
	if cfg == nil || closeC == nil {
		return nil
	}
	log := &singleLog{
		closeC:   closeC,
		filePath: cfg.FilePath,
		filename: cfg.Filename,
	}
	_ = log.initialize()
	return log
}
