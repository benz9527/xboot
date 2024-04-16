package xlog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ xLogCore = (*consoleCore)(nil)

type consoleCore struct{}

func (cc *consoleCore) Build(
	lvl zapcore.Level,
	encoder LogEncoderType,
	writer LogOutWriterType,
	lvlEnc zapcore.LevelEncoder,
	tsEnc zapcore.TimeEncoder,
) (core zapcore.Core, stop func() error, err error) {
	config := zapcore.EncoderConfig{
		MessageKey:    "msg",
		LevelKey:      "lvl",
		EncodeLevel:   lvlEnc,
		TimeKey:       "ts",
		EncodeTime:    tsEnc,
		CallerKey:     "callAt",
		EncodeCaller:  zapcore.ShortCallerEncoder,
		FunctionKey:   "fn",
		NameKey:       coreKeyIgnored,
		StacktraceKey: coreKeyIgnored,
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
