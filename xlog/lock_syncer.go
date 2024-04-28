package xlog

import (
	"context"
	"io"
	"sync"

	"go.uber.org/zap/zapcore"
)

var _ zapcore.WriteSyncer = (*xLogLockSyncer)(nil)

type xLogLockSyncer struct {
	ctx       context.Context
	outWriter io.WriteCloser
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

	select {
	case <-syncer.ctx.Done():
		return 0, io.EOF
	default:
	}

	// TODO Implemented filters or hooks to pre-process logs.
	return syncer.outWriter.Write(log)
}

func XLogLockSyncer(ctx context.Context, writer io.WriteCloser) zapcore.WriteSyncer {
	if writer == nil || ctx == nil {
		return nil
	}
	syncer := &xLogLockSyncer{
		ctx:       ctx,
		outWriter: writer,
	}
	return syncer
}
