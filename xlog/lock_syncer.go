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

	return syncer.outWriter.Write(log)
}

func (syncer *xLogLockSyncer) Stop() (err error) {
	close(syncer.closeC)
	return nil
}

func (syncer *xLogLockSyncer) stop() {
	select {
	case <-syncer.closeC:
		if _, ok := syncer.outWriter.(*rotateLog); !ok {
			_ = syncer.outWriter.Close() // Notice: data race !!!
		}
	}
}

func XLogLockSyncer(writer io.WriteCloser) zapcore.WriteSyncer {
	syncer := &xLogLockSyncer{
		outWriter: writer,
		closeC:    make(chan struct{}),
	}
	go syncer.stop()
	return syncer
}
