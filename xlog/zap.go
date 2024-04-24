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
	logger              atomic.Pointer[zap.Logger]
	ctxFields           kv.ThreadSafeStorer[string, string]
	dynamicLevelEnabler zap.AtomicLevel
	writer              logOutWriterType
	encoder             logEncoderType
}

func (l *xLogger) zap() *zap.Logger {
	return l.logger.Load()
}

// IncreaseLogLevel we can increase or decrease the log level concurrently.
func (l *xLogger) IncreaseLogLevel(level zapcore.Level) {
	l.dynamicLevelEnabler.SetLevel(level)
}

func (l *xLogger) Sync() error {
	return l.logger.Load().Sync()
}

func (l *xLogger) Level() string {
	return l.dynamicLevelEnabler.Level().String()
}

func (l *xLogger) Banner(banner Banner) {
	printBanner.Do(func() {
		var enc zapcore.Encoder
		core := zapcore.EncoderConfig{
			MessageKey:    "banner", // Required, but the plain text will be ignored.
			LevelKey:      coreKeyIgnored,
			EncodeLevel:   nil,
			TimeKey:       coreKeyIgnored,
			EncodeTime:    nil,
			CallerKey:     coreKeyIgnored,
			EncodeCaller:  nil,
			StacktraceKey: coreKeyIgnored,
		}
		switch l.encoder {
		case JSON:
			enc = zapcore.NewJSONEncoder(core)
		case PlainText:
			enc = zapcore.NewConsoleEncoder(core)
		}
		ws := getOutWriterByType(l.writer)
		lvlEnabler := zap.NewAtomicLevelAt(zapcore.InfoLevel)
		_l := l.logger.Load().WithOptions(
			zap.WrapCore(func(core zapcore.Core) zapcore.Core {
				return zapcore.NewCore(enc, ws, lvlEnabler)
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

func (l *xLogger) Log(lvl zapcore.Level, msg string, fields ...zap.Field) {
	l.logger.Load().Log(lvl, msg, fields...)
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
	ctxFields        kv.ThreadSafeStorer[string, string]
	writerType       *logOutWriterType
	encoderType      *logEncoderType
	lvlEncoder       zapcore.LevelEncoder
	tsEncoder        zapcore.TimeEncoder
	level            *zapcore.Level
	coreConstructors []XLogCoreConstructor
	cores            []zapcore.Core
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
		l.dynamicLevelEnabler = zap.NewAtomicLevelAt(*cfg.level)
	} else {
		l.dynamicLevelEnabler = zap.NewAtomicLevelAt(getLogLevelOrDefault(os.Getenv("XLOG_LVL")))
	}

	l.ctxFields = cfg.ctxFields

	if cfg.lvlEncoder == nil {
		cfg.lvlEncoder = zapcore.CapitalLevelEncoder
	}

	if cfg.tsEncoder == nil {
		cfg.tsEncoder = zapcore.ISO8601TimeEncoder
	}

	if cfg.coreConstructors == nil || len(cfg.coreConstructors) == 0 {
		cfg.coreConstructors = []XLogCoreConstructor{
			newConsoleCore,
		}
		l.writer = StdOut
	}
	cfg.cores = make([]zapcore.Core, 0, 16)
	for _, cc := range cfg.coreConstructors {
		cfg.cores = append(cfg.cores, cc(
			l.dynamicLevelEnabler,
			l.encoder,
			l.writer,
			cfg.lvlEncoder,
			cfg.tsEncoder,
		))
	}
}

type XLoggerOption func(*loggerCfg) error

func NewXLogger(opts ...XLoggerOption) XLogger {
	cfg := &loggerCfg{}
	for _, o := range opts {
		if err := o(cfg); err != nil {
			panic(err)
		}
	}
	xl := &xLogger{}
	cfg.apply(xl)

	// Disable zap logger error stack.
	l := zap.New(
		zapcore.NewTee(cfg.cores...),
		zap.AddCallerSkip(1), // Use caller filename as service
		zap.AddCaller(),
	)
	xl.logger.Store(l)
	return xl
}

func WithXLoggerWriter(w logOutWriterType) XLoggerOption {
	return func(cfg *loggerCfg) error {
		if w == _writerMax {
			return infra.NewErrorStack("unknown xlogger writer")
		}
		cfg.writerType = &w
		return nil
	}
}

func WithXLoggerEncoder(logEnc logEncoderType) XLoggerOption {
	return func(cfg *loggerCfg) error {
		if logEnc == _encMax {
			return infra.NewErrorStack("unknown xlogger encoder")
		}
		cfg.encoderType = &logEnc
		return nil
	}
}

func WithXLoggerLevel(lvl logLevel) XLoggerOption {
	return func(cfg *loggerCfg) error {
		_lvl := lvl.zapLevel()
		cfg.level = &_lvl
		return nil
	}
}

func WithXLoggerLevelEncoder(lvlEnc zapcore.LevelEncoder) XLoggerOption {
	return func(cfg *loggerCfg) error {
		if lvlEnc == nil {
			lvlEnc = zapcore.CapitalColorLevelEncoder
		}
		cfg.lvlEncoder = lvlEnc
		return nil
	}
}

func WithXLoggerTimeEncoder(tsEnc zapcore.TimeEncoder) XLoggerOption {
	return func(cfg *loggerCfg) error {
		if tsEnc == nil {
			tsEnc = zapcore.ISO8601TimeEncoder
		}
		cfg.tsEncoder = tsEnc
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
		if len(mapTo) == 0 || mapTo[0] == ContextKeyMapToItself {
			mapTo = []string{field}
		}
		return cfg.ctxFields.AddOrUpdate(field, mapTo[0])
	}
}

func WithXLoggerConsoleCore() XLoggerOption {
	return func(cfg *loggerCfg) error {
		if cfg.coreConstructors != nil || len(cfg.coreConstructors) <= 0 {
			cfg.coreConstructors = make([]XLogCoreConstructor, 0, 16)
		}
		cfg.coreConstructors = append(cfg.coreConstructors, newConsoleCore)
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
		if v == nil && mapTo != ContextKeyMapToOmitempty {
			newFields = append(newFields, zap.String(mapTo, "nil"))
		} else if v != nil && mapTo != ContextKeyMapToOmitempty {
			newFields = append(newFields, zap.Any(mapTo, v))
		}
	}
	return newFields
}
