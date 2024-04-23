package xlog

import (
	"io"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/benz9527/xboot/lib/infra"
	"github.com/benz9527/xboot/lib/list"
)

type logRecord struct {
	startOffset uint64
	length      uint64
}

type xLogArena struct {
	mu      sync.Mutex
	buf     []byte
	size    uint64
	wOffset uint64
	queue   list.LinkedList[*logRecord]
}

func (arena *xLogArena) availableBytes() uint64 {
	arena.mu.Lock()
	defer arena.mu.Unlock()
	return arena.size - arena.wOffset
}

func (arena *xLogArena) reset() {
	arena.mu.Lock()
	defer arena.mu.Unlock()
	if arena.wOffset == 0 {
		return
	}
	arena.wOffset = 0
}

func (arena *xLogArena) release() {
	arena.mu.Lock()
	defer arena.mu.Unlock()
	arena.reset()
	arena.buf = nil
	arena.queue = nil
}

func (arena *xLogArena) allocate(size uint64) (uint64, bool) {
	if arena.wOffset+size > arena.size {
		return 0, false // Flush first
	}
	arena.wOffset += size
	return /* startup */ arena.wOffset - size, true
}

func (arena *xLogArena) cache(log []byte) bool {
	arena.mu.Lock()
	defer arena.mu.Unlock()
	if arena.buf == nil || arena.queue == nil {
		return false
	}

	if offset, ok := arena.allocate(uint64(len(log))); ok {
		copy(arena.buf[offset:], log)
		_ = arena.queue.AppendValue(&logRecord{
			startOffset: offset,
			length:      uint64(len(log)),
		})
		return true
	}
	return false
}

func (arena *xLogArena) flush(writer io.WriteCloser) error {
	arena.mu.Lock()
	defer arena.mu.Unlock()
	if arena.queue == nil {
		return nil
	}

	err := arena.queue.Foreach(func(idx int64, e *list.NodeElement[*logRecord]) error {
		if _, err := writer.Write(arena.buf[e.Value.startOffset : e.Value.startOffset+e.Value.length]); err != nil {
			return err
		}
		arena.queue.Remove(e)
		return nil
	})
	if err != nil {
		return err
	}
	arena.reset()
	return nil
}

var _ zapcore.WriteSyncer = (*XLogBufferSyncer)(nil)

type XLogBufferSyncer struct {
	outWriter     io.WriteCloser
	flushInterval time.Duration
	arena         *xLogArena
	ticker        *time.Ticker
	closeC        chan struct{}
}

// Sync implements zapcore.WriteSyncer.
func (syncer *XLogBufferSyncer) Sync() error {
	return syncer.arena.flush(syncer.outWriter)
}

// Write implements zapcore.WriteSyncer.
func (x *XLogBufferSyncer) Write(log []byte) (n int, err error) {
	cached := x.arena.cache(log)
	if !cached {
		if err := x.arena.flush(x.outWriter); err != nil {
			return 0, err
		}
		if !x.arena.cache(log) {
			return 0, infra.NewErrorStack("[xlog] unable to cache log in buffer")
		}
	}
	return len(log), nil
}

func (syncer *XLogBufferSyncer) flushLoop() {
	for {
		select {
		case <-syncer.closeC:
			syncer.ticker.Stop()
			syncer.arena.release()
			return
		case <-syncer.ticker.C:
			_ = syncer.Sync()
		}
	}
}
