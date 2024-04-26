package xlog

import (
	"context"

	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ xLogCore = (xLogMultiCore)(nil)

type xLogMultiCore []xLogCore

func (mc xLogMultiCore) context() context.Context {
	return nil
}

func (mc xLogMultiCore) levelEncoder() zapcore.LevelEncoder {
	return nil
}

func (mc xLogMultiCore) outEncoder() func(cfg zapcore.EncoderConfig) zapcore.Encoder {
	return nil
}

func (mc xLogMultiCore) timeEncoder() zapcore.TimeEncoder {
	return nil
}

// writeSyncer implements xLogCore.
func (mc xLogMultiCore) writeSyncer() zapcore.WriteSyncer {
	return nil
}

func (mc xLogMultiCore) With(fields []zap.Field) zapcore.Core {
	clone := make([]zapcore.Core, len(mc))
	for i := range mc {
		clone[i] = mc[i].With(fields)
	}
	return zapcore.NewTee(clone...)
}

func (mc xLogMultiCore) Level() zapcore.Level {
	var minLvl zapcore.Level = -1
	for i := range mc {
		if lvl := zapcore.LevelOf(mc[i]); lvl < minLvl {
			minLvl = lvl
		}
	}
	return minLvl
}

func (mc xLogMultiCore) Enabled(lvl zapcore.Level) bool {
	for i := range mc {
		if mc[i].Enabled(lvl) {
			return true
		}
	}
	return false
}

func (mc xLogMultiCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	for i := range mc {
		ce = mc[i].Check(ent, ce)
	}
	return ce
}

func (mc xLogMultiCore) Write(ent zapcore.Entry, fields []zap.Field) error {
	var err error
	for i := range mc {
		err = multierr.Append(err, mc[i].Write(ent, fields))
	}
	return err
}

func (mc xLogMultiCore) Sync() error {
	var err error
	for i := range mc {
		err = multierr.Append(err, mc[i].Sync())
	}
	return err
}

func XLogTeeCore(cores ...xLogCore) xLogCore {
	return xLogMultiCore(cores)
}

func WrapCores(cores []xLogCore, cfg zapcore.EncoderConfig) (xLogCore, error) {
	newCores := make([]xLogCore, 0, len(cores))
	for i := range cores {
		newCore, err := WrapCore(cores[i], cfg)
		if err != nil {
			return nil, err
		}
		newCores = append(newCores, newCore)
	}
	return xLogMultiCore(newCores), nil
}

func WrapCoresNewLevelEnabler(cores []xLogCore, lvlEnabler zapcore.LevelEnabler, cfg zapcore.EncoderConfig) (xLogCore, error) {
	newCores := make([]xLogCore, 0, len(cores))
	for i := range cores {
		newCore, err := WrapCoreNewLevelEnabler(cores[i], lvlEnabler, cfg)
		if err != nil {
			return nil, err
		}
		newCores = append(newCores, newCore)
	}
	return xLogMultiCore(newCores), nil
}
