package xlog

import (
	"testing"
	"time"

	antsv2 "github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestAntsXLogger_ParentLogLevelChanged(t *testing.T) {
	var (
		parentLogger XLogger      = nil
		logger       *AntsXLogger = nil
	)
	logger.Printf("test %d", 123)

	opts := []XLoggerOption{
		WithXLoggerLevel(LogLevelDebug),
		WithXLoggerEncoder(JSON),
		WithXLoggerTimeEncoder(zapcore.ISO8601TimeEncoder),
		WithXLoggerLevelEncoder(zapcore.CapitalLevelEncoder),
	}
	parentLogger = NewXLogger(opts...)
	logger = NewAntsXLogger(parentLogger)
	parentLogger.IncreaseLogLevel(zapcore.InfoLevel)
	parentLogger.Debug("abc")
	logger.Printf("test %d", 123)
	parentLogger.IncreaseLogLevel(zapcore.DebugLevel)
	parentLogger.Debug("abc")
	logger.Printf("test %d", 123)
	_ = parentLogger.Sync()
}

func TestAntsXLogger_AntsPool(t *testing.T) {
	var (
		parentLogger XLogger      = nil
		logger       *AntsXLogger = nil
	)
	opts := []XLoggerOption{
		WithXLoggerLevel(LogLevelDebug),
		WithXLoggerEncoder(JSON),
		WithXLoggerTimeEncoder(zapcore.ISO8601TimeEncoder),
		WithXLoggerLevelEncoder(zapcore.CapitalLevelEncoder),
	}
	parentLogger = NewXLogger(opts...)
	logger = NewAntsXLogger(parentLogger)

	p, err := antsv2.NewPool(10, antsv2.WithLogger(logger))
	require.NoError(t, err)
	err = p.Submit(func() {
		parentLogger.Logf(LogLevelDebug.zapLevel(), "test %d", 123)
	})
	require.NoError(t, err)
	err = p.Submit(func() {
		panic("xlogger panic in ants pool")
	})
	time.Sleep(100 * time.Millisecond)
	_ = parentLogger.Sync()
}
