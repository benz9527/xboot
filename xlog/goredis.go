package xlog

import (
	"context"
	"fmt"
	"strings"

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
	log := fmt.Sprintf(format, v...)
	if strings.Contains(log, "failed") {
		l.logger.Logf(zapcore.ErrorLevel, log)
		return
	}
	l.logger.Logf(zapcore.InfoLevel, log)
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
			cc, ok := core.(xLogCore)
			if !ok {
				panic("[XLogger] core is not XLogCore")
			}
			var err error
			if mc, ok := cc.(*xLogMultiCore); ok && mc != nil {
				if cc, err = WrapCores(mc.cores, componentCoreEncoderCfg); err != nil {
					panic(err)
				}
			} else {
				if cc, err = WrapCore(cc, componentCoreEncoderCfg); err != nil {
					panic(err)
				}
			}
			return cc
		})),
	)
	return &GoRedisXLogger{
		logger: l,
	}
}
