package xlog

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/benz9527/xboot/lib/infra"
	"github.com/benz9527/xboot/lib/kv"
)

var printBanner = sync.Once{}

type xLogger struct {
	logger    atomic.Pointer[zap.Logger]
	level     zapcore.Level
	ctxFields kv.ThreadSafeStorer[string, string]
	writer    LogOutWriterType
	encoder   LogEncoderType
}

func (l *xLogger) Banner(banner Banner) {
	printBanner.Do(func() {
		var enc zapcore.Encoder
		core := zapcore.EncoderConfig{
			MessageKey:    "banner", // Required, but the plain text will be ignored.
			LevelKey:      "",
			EncodeLevel:   nil,
			TimeKey:       "",
			EncodeTime:    nil,
			CallerKey:     "",
			EncodeCaller:  nil,
			StacktraceKey: "",
		}
		switch l.encoder {
		case JSON:
			enc = zapcore.NewJSONEncoder(core)
		case PlainText:
			enc = zapcore.NewConsoleEncoder(core)
		}
		_l := l.logger.Load().WithOptions(
			zap.WrapCore(func(core zapcore.Core) zapcore.Core {
				return zapcore.NewCore(
					enc,
					getOutWriterByType(l.writer),
					zap.LevelEnablerFunc(func(level zapcore.Level) bool {
						return level >= l.level
					}),
				)
			}),
		)
		switch l.encoder {
		case JSON:
			_l.Info(banner.JSON())
		case PlainText:
			_l.Info(banner.PlainText())
		}
	})
}

func (l *xLogger) IncreaseLogLevel(level zapcore.Level) {
	logger := l.logger.Load().WithOptions(zap.IncreaseLevel(level))
	l.logger.Store(logger)
}

func (l *xLogger) Sync() error {
	return l.logger.Load().Sync()
}

func (l *xLogger) Debug(msg string, fields ...zap.Field) {
	l.logger.Load().Debug(msg, fields...)
}

func (l *xLogger) Info(msg string, fields ...zap.Field) {
	l.logger.Load().Info(msg, fields...)
}

func (l *xLogger) Warn(msg string, fields ...zap.Field) {
	l.logger.Load().Warn(msg, fields...)
}

func (l *xLogger) Error(err error, msg string, fields ...zap.Field) {
	newFields := []zap.Field{
		zap.String("error", err.Error()),
	}
	newFields = append(newFields, fields...)
	l.logger.Load().Error(msg, newFields...)
}

func (l *xLogger) ErrorStack(err error, msg string, fields ...zap.Field) {
	var newFields []zap.Field
	if es, ok := err.(infra.ErrorStack); ok && es != nil {
		newFields = []zap.Field{
			zap.Inline(es),
		}
	}
	newFields = append(newFields, fields...)
	l.logger.Load().Error(msg, newFields...)
}

func (l *xLogger) DebugContext(ctx context.Context, msg string, fields ...zap.Field) {
	newFields := extractFieldsFromContext(ctx, l.ctxFields)
	newFields = append(newFields, fields...)
	l.logger.Load().Debug(msg, newFields...)
}

func (l *xLogger) InfoContext(ctx context.Context, msg string, fields ...zap.Field) {
	newFields := extractFieldsFromContext(ctx, l.ctxFields)
	newFields = append(newFields, fields...)
	l.logger.Load().Info(msg, newFields...)
}

func (l *xLogger) WarnContext(ctx context.Context, msg string, fields ...zap.Field) {
	newFields := extractFieldsFromContext(ctx, l.ctxFields)
	newFields = append(newFields, fields...)
	l.logger.Load().Warn(msg, newFields...)
}

func (l *xLogger) ErrorContext(ctx context.Context, err error, msg string, fields ...zap.Field) {
	newFields := extractFieldsFromContext(ctx, l.ctxFields)
	newFields = append(newFields, zap.String("error", err.Error()))
	newFields = append(newFields, fields...)
	l.logger.Load().Error(msg, newFields...)
}

func (l *xLogger) ErrorStackContext(ctx context.Context, err error, msg string, fields ...zap.Field) {
	newFields := extractFieldsFromContext(ctx, l.ctxFields)
	if es, ok := err.(infra.ErrorStack); ok && es != nil {
		newFields = append(newFields, zap.Inline(es))
	}
	newFields = append(newFields, fields...)
	l.logger.Load().Error(msg, newFields...)
}

func (l *xLogger) Logf(lvl zapcore.Level, format string, args ...any) {
	l.logger.Load().Log(lvl, fmt.Sprintf(format, args...))
}

func (l *xLogger) ErrorStackf(err error, format string, args ...any) {
	var newFields []zap.Field
	if es, ok := err.(infra.ErrorStack); ok && es != nil {
		newFields = []zap.Field{
			zap.Inline(es),
		}
	}
	l.logger.Load().Log(zap.ErrorLevel, fmt.Sprintf(format, args...), newFields...)
}

type loggerCfg struct {
	ctxFields   kv.ThreadSafeStorer[string, string]
	writerType  *LogOutWriterType
	encoderType *LogEncoderType
	level       *zapcore.Level
	core        xLogCore
}

func (cfg *loggerCfg) apply(l *xLogger) {
	if cfg.writerType != nil {
		l.writer = *cfg.writerType
	} else {
		l.writer = StdOut
	}

	if cfg.encoderType != nil {
		l.encoder = *cfg.encoderType
	} else {
		l.encoder = JSON
	}

	if cfg.level != nil {
		l.level = *cfg.level
	} else {
		l.level = getLogLevelOrDefault(os.Getenv("XLOG_LVL"))
	}

	l.ctxFields = cfg.ctxFields

	if cfg.core == nil {
		cfg.core = consoleCore{}
	}
}

type XLoggerOption func(*loggerCfg) error

func NewXLogger(opts ...XLoggerOption) *xLogger {
	cfg := &loggerCfg{}
	for _, o := range opts {
		if err := o(cfg); err != nil {
			panic(err)
		}
	}
	xl := &xLogger{}
	cfg.apply(xl)

	core, err := cfg.core.build(xl.level, xl.encoder, xl.writer)
	if err != nil {
		panic(err)
	}

	// Disable zap logger error stack.
	l := zap.New(
		zapcore.NewTee(core),
		zap.AddCallerSkip(1), // Use caller filename as service
		zap.AddCaller(),
	)
	xl.logger.Store(l)
	return xl
}

func WithXLoggerWriter(w LogOutWriterType) XLoggerOption {
	return func(cfg *loggerCfg) error {
		if w == _writerMax {
			return infra.NewErrorStack("unknown xlogger writer")
		}
		cfg.writerType = &w
		return nil
	}
}

func WithXLoggerEncoder(e LogEncoderType) XLoggerOption {
	return func(cfg *loggerCfg) error {
		if e == _encMax {
			return infra.NewErrorStack("unknown xlogger encoder")
		}
		cfg.encoderType = &e
		return nil
	}
}

func WithXLoggerLevel(lvl LogLevel) XLoggerOption {
	return func(cfg *loggerCfg) error {
		_lvl := lvl.zapLevel()
		cfg.level = &_lvl
		return nil
	}
}

func WithXLoggerContextFieldExtract(field string, mapTo ...string) XLoggerOption {
	return func(cfg *loggerCfg) error {
		if len(field) == 0 {
			return nil
		}
		if cfg.ctxFields == nil {
			cfg.ctxFields = kv.NewThreadSafeMap[string, string]()
		}
		if len(mapTo) == 0 || mapTo[0] == "" {
			mapTo = []string{field}
		}
		return cfg.ctxFields.AddOrUpdate(field, mapTo[0])
	}
}

func WithXLoggerConsoleCore() XLoggerOption {
	return func(cfg *loggerCfg) error {
		cfg.core = &consoleCore{}
		return nil
	}
}

func getLogLevelOrDefault(level string) zapcore.Level {
	if len(strings.TrimSpace(level)) == 0 {
		return zapcore.DebugLevel
	}

	switch strings.ToUpper(level) {
	case LogLevelInfo.String():
		return zapcore.InfoLevel
	case LogLevelWarn.String():
		return zapcore.WarnLevel
	case LogLevelError.String():
		return zapcore.ErrorLevel
	case LogLevelDebug.String():
		fallthrough
	default:
	}
	return zapcore.DebugLevel
}

func extractFieldsFromContext(
	ctx context.Context,
	targets kv.ThreadSafeStorer[string, string],
) []zap.Field {
	if ctx == nil || targets == nil {
		return []zap.Field{}
	}

	keys := targets.ListKeys()
	sort.StringSlice(keys).Sort()
	newFields := make([]zap.Field, 0, len(keys))
	for _, key := range keys {
		v := ctx.Value(key)
		mapTo, _ := targets.Get(key)
		if v == nil && mapTo != "_" { // Underline means omitempty.
			newFields = append(newFields, zap.String(mapTo, "nil"))
		} else if v != nil && mapTo != "_" {
			newFields = append(newFields, zap.Any(mapTo, v))
		}
	}
	return newFields
}
