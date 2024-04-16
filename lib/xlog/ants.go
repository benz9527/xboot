package xlog

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type AntsXLogger struct {
	logger XLogger
}

func (l *AntsXLogger) Printf(format string, args ...any) {
	if l == nil {
		return
	}
	l.logger.Debug(fmt.Sprintf(format, args...))
}

func NewAntsXLogger(logger XLogger) *AntsXLogger {
	l := &xLogger{}
	l.logger.Store(logger.
		zap().
		Named("Ants").
		WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			config := zapcore.EncoderConfig{
				MessageKey:    "msg",
				LevelKey:      "lvl",
				EncodeLevel:   logger.levelEncoder(),
				TimeKey:       "ts",
				EncodeTime:    logger.timeEncoder(),
				CallerKey:     coreKeyIgnored,
				EncodeCaller:  zapcore.ShortCallerEncoder,
				FunctionKey:   coreKeyIgnored,
				NameKey:       "component",
				EncodeName:    zapcore.FullNameEncoder,
				StacktraceKey: coreKeyIgnored,
			}
			return zapcore.NewCore(
				logger.outEncoder()(config),
				logger.writeSyncer(),
				logger.levelEnablerFunc(),
			)
		})),
	)
	return &AntsXLogger{
		logger: l,
	}
}
