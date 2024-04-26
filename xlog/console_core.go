package xlog

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ xLogCore = (*consoleCore)(nil)

type consoleCore struct {
	core *commonCore
}

func (cc *consoleCore) timeEncoder() zapcore.TimeEncoder   { return cc.core.tsEnc }
func (cc *consoleCore) levelEncoder() zapcore.LevelEncoder { return cc.core.lvlEnc }
func (cc *consoleCore) writeSyncer() zapcore.WriteSyncer   { return cc.core.ws }
func (cc *consoleCore) outEncoder() func(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return cc.core.enc
}
func (cc *consoleCore) context() context.Context             { return cc.core.ctx }
func (cc *consoleCore) Enabled(lvl zapcore.Level) bool       { return cc.core.lvlEnabler.Enabled(lvl) }
func (cc *consoleCore) With(fields []zap.Field) zapcore.Core { return cc.core.With(fields) }
func (cc *consoleCore) Sync() error                          { return cc.core.Sync() }
func (cc *consoleCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return cc.core.Check(ent, ce)
}

func (cc *consoleCore) Write(ent zapcore.Entry, fields []zap.Field) error {
	return cc.core.Write(ent, fields)
}

func newConsoleCore(
	ctx context.Context,
	lvlEnabler zapcore.LevelEnabler,
	encoder logEncoderType,
	lvlEnc zapcore.LevelEncoder,
	tsEnc zapcore.TimeEncoder,
) xLogCore {
	if ctx == nil {
		return nil
	}
	cc := &consoleCore{
		core: &commonCore{
			ctx:        ctx,
			lvlEnabler: lvlEnabler,
			lvlEnc:     lvlEnc,
			tsEnc:      tsEnc,
			ws:         getOutWriterByType(StdOut),
			enc:        getEncoderByType(encoder),
		},
	}
	config := zapcore.EncoderConfig{
		MessageKey:    "msg",
		LevelKey:      "lvl",
		EncodeLevel:   cc.core.lvlEnc,
		TimeKey:       "ts",
		EncodeTime:    cc.core.tsEnc,
		CallerKey:     "callAt",
		EncodeCaller:  zapcore.ShortCallerEncoder,
		FunctionKey:   "fn",
		NameKey:       "component",
		EncodeName:    zapcore.FullNameEncoder,
		StacktraceKey: coreKeyIgnored,
	}
	cc.core.core = zapcore.NewCore(cc.core.enc(config), cc.core.ws, cc.core.lvlEnabler)
	return cc
}
