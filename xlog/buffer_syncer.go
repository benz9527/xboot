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
	buf     []byte
	size    uint64
	wOffset uint64
	queue   list.LinkedList[logRecord]
}

func (arena *xLogArena) availableBytes() uint64 {
	return arena.size - arena.wOffset
}

func (arena *xLogArena) reset() {
	if arena.wOffset == 0 {
		return
	}
	arena.wOffset = 0
}

func (arena *xLogArena) release() {
	arena.reset()
	arena.buf = nil
	arena.queue = nil
}

func (arena *xLogArena) allocate(size uint64) (uint64, bool) {
	if arena.availableBytes() < size {
		return 0, false // Flush first
	}
	arena.wOffset += size
	return /* startup */ arena.wOffset - size, true
}

func (arena *xLogArena) cache(log []byte) bool {
	if arena.buf == nil || arena.queue == nil {
		return false
	}

	if offset, ok := arena.allocate(uint64(len(log))); ok {
		copy(arena.buf[offset:], log)
		_ = arena.queue.AppendValue(logRecord{
			startOffset: offset,
			length:      uint64(len(log)),
		})
		return true
	}
	return false
}

func (arena *xLogArena) flush(writer io.WriteCloser) error {
	if arena.queue == nil {
		return nil
	}

	// TODO Batch bytes write in one io write.
	err := arena.queue.Foreach(func(idx int64, e *list.NodeElement[logRecord]) error {
		data := arena.buf[e.Value.startOffset : e.Value.startOffset+e.Value.length]
		if _, err := writer.Write(data); err != nil {
			return err
		}
		arena.queue.Remove(e)
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

const (
	defaultBufferSize    = 512 << 10
	defaultFlushInterval = 30 * time.Second
)

var _ XLogCloseableWriteSyncer = (*xLogBufferSyncer)(nil)

type xLogBufferSyncer struct {
	outWriter     io.WriteCloser
	flushInterval time.Duration
	arena         *xLogArena
	clock         zapcore.Clock
	ticker        *time.Ticker
	closeC        chan struct{}
	once          sync.Once
	mu            sync.Mutex
}

// Sync implements zapcore.WriteSyncer.
func (syncer *xLogBufferSyncer) Sync() error {
	syncer.mu.Lock()
	defer syncer.mu.Unlock()

	err := syncer.arena.flush(syncer.outWriter)
	if err != nil {
		return err
	}
	syncer.arena.reset()
	return nil
}

// Write implements zapcore.WriteSyncer.
func (syncer *xLogBufferSyncer) Write(log []byte) (n int, err error) {
	syncer.mu.Lock()
	defer syncer.mu.Unlock()

	if !syncer.arena.cache(log) {
		if err := syncer.arena.flush(syncer.outWriter); err != nil {
			return 0, err
		}
		syncer.arena.reset()
		if !syncer.arena.cache(log) {
			if /* too long to cache */ _, err := syncer.outWriter.Write(log); err != nil {
				return 0, infra.NewErrorStack("[xlog-buf-syncer] unable to cache log in buffer")
			}
		}
	}
	return len(log), nil
}

func (syncer *xLogBufferSyncer) Stop() (err error) {
	close(syncer.closeC)
	return nil
}

func (syncer *xLogBufferSyncer) flushLoop() {
	for {
		select {
		case <-syncer.closeC:
			_ = syncer.Sync()
			syncer.ticker.Stop()
			syncer.arena.release()
			if _, ok := syncer.outWriter.(*rotateLog); !ok {
				_ = syncer.outWriter.Close() // Notice: data race !!!
			}
			return
		case <-syncer.ticker.C:
			_ = syncer.Sync()
		}
	}
}

func (syncer *xLogBufferSyncer) initialize() {
	syncer.once.Do(func() {
		if syncer.arena == nil || syncer.arena.size == 0 {
			syncer.arena = &xLogArena{
				size:  defaultBufferSize,
				buf:   make([]byte, defaultBufferSize),
				queue: list.NewLinkedList[logRecord](),
			}
		} else if syncer.arena.size > 0 {
			syncer.arena.buf = make([]byte, syncer.arena.size)
			syncer.arena.queue = list.NewLinkedList[logRecord]()
		}

		if syncer.flushInterval == 0 {
			syncer.flushInterval = defaultFlushInterval
		}

		if syncer.clock == nil {
			syncer.clock = zapcore.DefaultClock
		}

		if syncer.ticker == nil {
			syncer.ticker = syncer.clock.NewTicker(syncer.flushInterval)
		}

		if syncer.closeC == nil {
			syncer.closeC = make(chan struct{})
		}

		if rl, ok := syncer.outWriter.(*rotateLog); ok && rl != nil {
			rl.closeC = syncer.closeC
			if err := rl.initialize(); err != nil {
				panic(err)
			}
		}

		go syncer.flushLoop()
	})
}

func XLogBufferSyncer(
	writer io.WriteCloser,
	bufSize uint64,
	flushInterval time.Duration,
) zapcore.WriteSyncer {
	syncer := &xLogBufferSyncer{
		outWriter: writer,
		arena: &xLogArena{
			size: bufSize,
		},
		flushInterval: time.Duration(flushInterval) * time.Millisecond,
	}
	syncer.initialize()
	return syncer
}
