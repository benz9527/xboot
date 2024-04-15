package xlog

import (
	"context"
	"os"

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

var (
	writerMap = map[LogOutWriterType]zapcore.WriteSyncer{
		StdOut: zapcore.Lock(os.Stdout),
	}
	encoderMap = map[LogEncoderType]func(cfg zapcore.EncoderConfig) zapcore.Encoder{
		JSON:      zapcore.NewJSONEncoder,
		PlainText: zapcore.NewConsoleEncoder,
	}
)

func getEncoderByType(typ LogEncoderType) func(cfg zapcore.EncoderConfig) zapcore.Encoder {
	enc, ok := encoderMap[typ]
	if !ok {
		return zapcore.NewJSONEncoder
	}
	return enc
}

func getOutWriterByType(typ LogOutWriterType) zapcore.WriteSyncer {
	out, ok := writerMap[typ]
	if !ok {
		return zapcore.Lock(os.Stdout)
	}
	return out
}

type Banner interface {
	JSON() string
	PlainText() string
}

type xLogCore interface {
	build(lvl zapcore.Level, encoder LogEncoderType, writer LogOutWriterType) (core zapcore.Core, err error)
}

type XLogger interface {
	IncreaseLogLevel(level zapcore.Level)
	Sync() error
	Banner(banner Banner)

	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(err error, msg string, fields ...zap.Field)

	// ErrorStack is used to print all errors throws stacks.
	// Instead of using zap default error stack, it can print
	// the error stack in JSON format. It is easy for us to
	// use fluentd, fluentbit or other log aggregator to
	// parse the error stack, then display them in elastic
	// search or other tools.
	ErrorStack(err error, msg string, fields ...zap.Field)

	DebugContext(ctx context.Context, msg string, fields ...zap.Field)
	InfoContext(ctx context.Context, msg string, fields ...zap.Field)
	WarnContext(ctx context.Context, msg string, fields ...zap.Field)
	ErrorContext(ctx context.Context, err error, msg string, fields ...zap.Field)
	ErrorStackContext(ctx context.Context, err error, msg string, fields ...zap.Field)

	Logf(lvl zapcore.Level, format string, args ...any)
	ErrorStackf(err error, format string, args ...any)
}
