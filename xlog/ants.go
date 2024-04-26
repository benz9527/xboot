package xlog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type AntsXLogger struct {
	logger XLogger
}

func (l *AntsXLogger) Printf(format string, args ...any) {
	if l == nil || l.logger == nil {
		return
	}
	l.logger.Logf(zapcore.ErrorLevel, format, args...)
}

func NewAntsXLogger(logger XLogger) *AntsXLogger {
	l := &xLogger{}
	l.logger.Store(logger.
		zap().
		Named("Ants").
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
	return &AntsXLogger{
		logger: l,
	}
}
