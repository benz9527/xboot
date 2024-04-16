package xlog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ xLogCore = (*consoleCore)(nil)

type consoleCore struct{}

func (cc *consoleCore) build(lvl zapcore.Level, encoder LogEncoderType, writer LogOutWriterType) (core zapcore.Core, stop func() error, err error) {
	config := zapcore.EncoderConfig{
		MessageKey:    "msg",
		LevelKey:      "lvl",
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		TimeKey:       "ts",
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		CallerKey:     "callAt",
		EncodeCaller:  zapcore.ShortCallerEncoder,
		StacktraceKey: "stack",
	}
	levelFn := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level >= lvl
	})
	ws, stop := getOutWriterByType(writer)
	core = zapcore.NewCore(
		getEncoderByType(encoder)(config),
		ws,
		levelFn,
	)
	return core, stop, nil
}
