package xlog

import (
	"go.uber.org/zap/zapcore"
)

var _ XLogCore = (*consoleCore)(nil)

type consoleCore struct{}

func (cc *consoleCore) Build(
	lvlEnabler zapcore.LevelEnabler,
	encoder LogEncoderType,
	writer LogOutWriterType,
	lvlEnc zapcore.LevelEncoder,
	tsEnc zapcore.TimeEncoder,
) (core zapcore.Core, err error) {
	config := zapcore.EncoderConfig{
		MessageKey:    "msg",
		LevelKey:      "lvl",
		EncodeLevel:   lvlEnc,
		TimeKey:       "ts",
		EncodeTime:    tsEnc,
		CallerKey:     "callAt",
		EncodeCaller:  zapcore.ShortCallerEncoder,
		FunctionKey:   "fn",
		NameKey:       "component",
		EncodeName:    zapcore.FullNameEncoder,
		StacktraceKey: coreKeyIgnored,
	}
	ws := getOutWriterByType(writer)
	core = zapcore.NewCore(getEncoderByType(encoder)(config), ws, lvlEnabler)
	return core, nil
}
