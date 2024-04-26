package xlog

import (
	"io"
	"sync"

	"go.uber.org/zap/zapcore"
)

var _ XLogCloseableWriteSyncer = (*xLogLockSyncer)(nil)

type xLogLockSyncer struct {
	outWriter io.WriteCloser
	closeC    chan struct{}
	mu        sync.Mutex
}

// Sync implements zapcore.WriteSyncer.
func (syncer *xLogLockSyncer) Sync() error {
	return nil
}

// Write implements zapcore.WriteSyncer.
func (syncer *xLogLockSyncer) Write(log []byte) (n int, err error) {
	syncer.mu.Lock()
	defer syncer.mu.Unlock()

	// TODO Implemented filters or hooks to pre-process logs.
	return syncer.outWriter.Write(log)
}

func (syncer *xLogLockSyncer) Stop() (err error) {
	close(syncer.closeC)
	return nil
}

func XLogLockSyncer(writer io.WriteCloser, closeC chan struct{}) zapcore.WriteSyncer {
	if writer == nil || closeC == nil {
		return nil
	}
	syncer := &xLogLockSyncer{
		outWriter: writer,
		closeC:    closeC,
	}
	return syncer
}
