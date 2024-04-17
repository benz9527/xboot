package xlog

import (
	"testing"

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
		WithXLoggerWriter(StdOut),
		WithXLoggerConsoleCore(),
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
