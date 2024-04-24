package xlog

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type GoRedisXLogger struct {
	logger XLogger
}

func (l *GoRedisXLogger) Printf(ctx context.Context, format string, v ...any) {
	if l == nil || l.logger == nil {
		return
	}
	l.logger.Logf(zapcore.DebugLevel, format, v...)
}

func NewGoRedisXLogger(logger XLogger) *GoRedisXLogger {
	l := &xLogger{}
	l.logger.Store(logger.
		zap().
		Named("GoRedis").
		WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
			if core == nil {
				panic("[XLogger] core is nil")
			}
			cc, ok := core.(XLogCore)
			if !ok {
				panic("[XLogger] core is not XLogCore")
			}
			var err error
			if cc, err = WrapCore(cc, componentCoreEncoderCfg); err != nil {
				panic(err)
			}
			return cc
		})),
	)
	return &GoRedisXLogger{
		logger: l,
	}
}
