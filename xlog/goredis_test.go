package xlog

import (
	"context"
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestGoRedisXLogger(t *testing.T) {
	var (
		parentLogger XLogger         = nil
		logger       *GoRedisXLogger = nil
	)
	logger.Printf(context.TODO(), "test %d", 123)

	opts := []XLoggerOption{
		WithXLoggerLevel(LogLevelDebug),
		WithXLoggerEncoder(JSON),
		WithXLoggerWriter(StdOut),
		WithXLoggerConsoleCore(),
		WithXLoggerTimeEncoder(zapcore.ISO8601TimeEncoder),
		WithXLoggerLevelEncoder(zapcore.CapitalLevelEncoder),
	}
	parentLogger = NewXLogger(opts...)
	logger = NewGoRedisXLogger(parentLogger)
	parentLogger.IncreaseLogLevel(zapcore.InfoLevel)
	parentLogger.Debug("abc")
	logger.Printf(context.TODO(), "test %d", 123)
	parentLogger.IncreaseLogLevel(zapcore.DebugLevel)
	parentLogger.Debug("abc")
	logger.Printf(context.TODO(), "test %d", 123)
	_ = parentLogger.Sync()
}
