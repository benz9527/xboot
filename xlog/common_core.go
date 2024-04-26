package xlog

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/benz9527/xboot/lib/infra"
)

var _ xLogCore = (*commonCore)(nil)

type commonCore struct {
	ctx        context.Context
	lvlEnabler zapcore.LevelEnabler
	lvlEnc     zapcore.LevelEncoder
	tsEnc      zapcore.TimeEncoder
	ws         zapcore.WriteSyncer
	enc        func(cfg zapcore.EncoderConfig) zapcore.Encoder
	core       zapcore.Core
}

func (cc *commonCore) timeEncoder() zapcore.TimeEncoder                            { return cc.tsEnc }
func (cc *commonCore) levelEncoder() zapcore.LevelEncoder                          { return cc.lvlEnc }
func (cc *commonCore) writeSyncer() zapcore.WriteSyncer                            { return cc.ws }
func (cc *commonCore) outEncoder() func(cfg zapcore.EncoderConfig) zapcore.Encoder { return cc.enc }
func (cc *commonCore) context() context.Context                                    { return cc.ctx }
func (cc *commonCore) Enabled(lvl zapcore.Level) bool {
	return cc.lvlEnabler.Enabled(lvl)
}

func (cc *commonCore) With(fields []zap.Field) zapcore.Core {
	return cc.core.With(fields)
}

func (cc *commonCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return cc.core.Check(ent, ce)
}

func (cc *commonCore) Write(ent zapcore.Entry, fields []zap.Field) error {
	return cc.core.Write(ent, fields)
}

func (cc *commonCore) Sync() error {
	return cc.core.Sync()
}

func WrapCore(core xLogCore, cfg *zapcore.EncoderConfig) (xLogCore, error) {
	if cfg == nil {
		return nil, infra.NewErrorStack("[XLogger] logger core config is empty")
	}
	cfg.EncodeLevel = core.levelEncoder()
	cfg.EncodeTime = core.timeEncoder()

	cc := &commonCore{
		ctx: core.context(),
		ws:  core.writeSyncer(),
		enc: core.outEncoder(),
		lvlEnabler: zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			return core.Enabled(l)
		}),
		lvlEnc: core.levelEncoder(),
		tsEnc:  core.timeEncoder(),
	}
	cc.core = zapcore.NewCore(core.outEncoder()(*cfg), core.writeSyncer(), cc.lvlEnabler)
	return cc, nil
}

func WrapCoreNewLevelEnabler(core xLogCore, lvlEnabler zapcore.LevelEnabler, cfg *zapcore.EncoderConfig) (xLogCore, error) {
	if cfg == nil {
		return nil, infra.NewErrorStack("[XLogger] logger core config is empty")
	}
	cfg.EncodeLevel = core.levelEncoder()
	cfg.EncodeTime = core.timeEncoder()

	cc := &commonCore{
		ws:  core.writeSyncer(),
		enc: core.outEncoder(),
		lvlEnabler: zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			return lvlEnabler.Enabled(l)
		}),
		lvlEnc: core.levelEncoder(),
		tsEnc:  core.timeEncoder(),
	}
	cc.core = zapcore.NewCore(core.outEncoder()(*cfg), core.writeSyncer(), cc.lvlEnabler)
	return cc, nil
}

var componentCoreEncoderCfg = &zapcore.EncoderConfig{
	MessageKey:    "msg",
	LevelKey:      "lvl",
	TimeKey:       "ts",
	CallerKey:     coreKeyIgnored,
	EncodeCaller:  zapcore.ShortCallerEncoder,
	FunctionKey:   coreKeyIgnored,
	NameKey:       "component",
	EncodeName:    zapcore.FullNameEncoder,
	StacktraceKey: coreKeyIgnored,
}
