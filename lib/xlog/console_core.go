package xlog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ xLogCore = consoleCore{}

type consoleCore struct{}

func (cc consoleCore) build(lvl zapcore.Level, encoder LogEncoderType, writer LogOutWriterType) (core zapcore.Core, err error) {
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
	core = zapcore.NewCore(
		getEncoderByType(encoder)(config),
		getOutWriterByType(writer),
		levelFn,
	)
	return core, nil
}
