package xlog

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/benz9527/xboot/lib/kv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
)

func (lvl LogLevel) zapLevel() zapcore.Level {
	switch lvl {
	case LogLevelDebug:
		return zapcore.DebugLevel
	case LogLevelInfo:
		return zapcore.InfoLevel
	case LogLevelWarn:
		return zapcore.WarnLevel
	case LogLevelError:
		return zapcore.ErrorLevel
	default:
	}
	return zapcore.DebugLevel
}

func (lvl LogLevel) String() string {
	return string(lvl)
}

type LogEncoderType uint8

const (
	JSON LogEncoderType = iota
	PlainText
	_encMax
)

type LogOutWriterType uint8

const (
	StdOut LogOutWriterType = iota
	testMemAsOut
	_writerMax
)

const (
	ContextKeyMapToOmitempty = "_"
	ContextKeyMapToItself    = ""
	coreKeyIgnored           = ""
)

var (
	writerMap  = kv.NewSwissMap[LogOutWriterType, zapcore.WriteSyncer](16)
	encoderMap = map[LogEncoderType]func(cfg zapcore.EncoderConfig) zapcore.Encoder{
		JSON:      zapcore.NewJSONEncoder,
		PlainText: zapcore.NewConsoleEncoder,
	}
)

func init() {
	_ = writerMap.Put(StdOut, &zapcore.BufferedWriteSyncer{WS: os.Stdout, Size: 512 * 1024, FlushInterval: 30 * time.Second})
	runtime.SetFinalizer(writerMap, func(w kv.Map[LogOutWriterType, zapcore.WriteSyncer]) {
		// May be useless to release the buffer.
		ws, ok := w.Get(StdOut)
		if !ok {
			return
		}
		if _ws, ok := ws.(*zapcore.BufferedWriteSyncer); ok {
			_ = _ws.Stop()
		}
	})
}

func getEncoderByType(typ LogEncoderType) func(cfg zapcore.EncoderConfig) zapcore.Encoder {
	enc, ok := encoderMap[typ]
	if !ok {
		return zapcore.NewJSONEncoder
	}
	return enc
}

func getOutWriterByType(typ LogOutWriterType) zapcore.WriteSyncer {
	out, ok := writerMap.Get(typ)
	if !ok {
		return zapcore.Lock(os.Stdout)
	}
	return out
}

type Banner interface {
	JSON() string
	PlainText() string
}

type XLogCore interface {
	Build(
		zapcore.LevelEnabler,
		LogEncoderType,
		LogOutWriterType,
		zapcore.LevelEncoder,
		zapcore.TimeEncoder,
	) (core zapcore.Core, err error)
}

// XLogger mainly implemented by Uber zap logger.
//
// zap(), timeEncoder(), levelEncoder(), writeSyncer(),
// levelEnablerFunc(), outEncoder() are used to create
// child logger which will redefine the zapcore.Core.
//
// ErrorStack is used to print all errors throws stacks.
// Instead of using zap default error stack, it can print
// the error stack in JSON format. It is easy for us to
// use fluentd, fluentbit or other log aggregator to
// parse the error stack, then display them in elastic
// search or other tools.
//
// The interface methods with context is used to add more
// additional fields to the log. We can pass like trace ID,
// service name, etc. To do the log trace.
//
// Log format is not recommended, because it is low performance.
type XLogger interface {
	zap() *zap.Logger
	timeEncoder() zapcore.TimeEncoder
	levelEncoder() zapcore.LevelEncoder
	writeSyncer() zapcore.WriteSyncer
	levelEnablerFunc() zap.LevelEnablerFunc
	outEncoder() func(cfg zapcore.EncoderConfig) zapcore.Encoder

	IncreaseLogLevel(level zapcore.Level)
	Level() string
	Sync() error
	Banner(banner Banner)

	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(err error, msg string, fields ...zap.Field)
	ErrorStack(err error, msg string, fields ...zap.Field)

	DebugContext(ctx context.Context, msg string, fields ...zap.Field)
	InfoContext(ctx context.Context, msg string, fields ...zap.Field)
	WarnContext(ctx context.Context, msg string, fields ...zap.Field)
	ErrorContext(ctx context.Context, err error, msg string, fields ...zap.Field)
	ErrorStackContext(ctx context.Context, err error, msg string, fields ...zap.Field)

	Logf(lvl zapcore.Level, format string, args ...any)
	ErrorStackf(err error, format string, args ...any)
}
