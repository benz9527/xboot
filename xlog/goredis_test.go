package xlog

import (
	"context"
	"testing"
	"time"

	mredisv2 "github.com/alicebob/miniredis/v2"
	redisv9 "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestGoRedisXLogger_ParentLogLevelChanged(t *testing.T) {
	var (
		parentLogger XLogger         = nil
		logger       *GoRedisXLogger = nil
	)
	logger.Printf(context.TODO(), "test %d", 123)

	opts := []XLoggerOption{
		WithXLoggerLevel(LogLevelDebug),
		WithXLoggerEncoder(JSON),
		WithXLoggerTimeEncoder(zapcore.ISO8601TimeEncoder),
		WithXLoggerLevelEncoder(zapcore.CapitalLevelEncoder),
	}
	parentLogger = NewXLogger(opts...)
	logger = NewGoRedisXLogger(parentLogger)
	parentLogger.IncreaseLogLevel(zapcore.ErrorLevel)
	parentLogger.Debug("abc")
	logger.Printf(context.TODO(), "test %d", 123)
	parentLogger.IncreaseLogLevel(zapcore.DebugLevel)
	parentLogger.Debug("abc")
	logger.Printf(context.TODO(), "test %d", 123)
	logger.Printf(context.TODO(), "test failed: %d", 123)
	_ = parentLogger.Sync()
	require.Panics(t, func() {
		parentLogger = &xLogger{}
		logger = NewGoRedisXLogger(parentLogger)
	})
}

func TestGoRedisXLogger_MiniRedis(t *testing.T) {
	var (
		parentLogger XLogger         = nil
		logger       *GoRedisXLogger = nil
	)
	opts := []XLoggerOption{
		WithXLoggerLevel(LogLevelDebug),
		WithXLoggerEncoder(JSON),
		WithXLoggerTimeEncoder(zapcore.ISO8601TimeEncoder),
		WithXLoggerLevelEncoder(zapcore.CapitalLevelEncoder),
	}
	parentLogger = NewXLogger(opts...)
	logger = NewGoRedisXLogger(parentLogger)

	const addr = "127.0.0.1:6379"
	mredis := mredisv2.NewMiniRedis()
	defer func() { mredis.Close() }()
	go func() {
		err := mredis.StartAddr(addr)
		require.NoError(t, err)
	}()

	redisv9.SetLogger(logger)
	rclient := redisv9.NewClient(&redisv9.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})
	defer func() { _ = rclient.Close() }()
	rclient.Set(context.TODO(), "abc", "123", 0)
	cmd := rclient.BLPop(context.TODO(), 1*time.Millisecond, "abc")
	_, err := cmd.Result()
	require.Error(t, err)
	parentLogger.Debug("go redis", zap.Any("cmd", cmd.String()))
	_ = parentLogger.Sync()
}
