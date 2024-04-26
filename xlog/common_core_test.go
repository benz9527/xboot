package xlog

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestCommonCore(t *testing.T) {
	var cc xLogCore = &commonCore{}
	require.Nil(t, cc.outEncoder())
	require.Nil(t, cc.writeSyncer())
	require.Nil(t, cc.levelEncoder())
	require.Nil(t, cc.timeEncoder())
	require.Nil(t, cc.(*commonCore).lvlEnabler)
	require.Nil(t, cc.(*commonCore).core)

	lvlEnabler := zap.NewAtomicLevelAt(LogLevelDebug.zapLevel())
	cc = &commonCore{
		lvlEnabler: &lvlEnabler,
		lvlEnc:     zapcore.CapitalLevelEncoder,
		tsEnc:      zapcore.ISO8601TimeEncoder,
		ws:         getOutWriterByType(logOutWriterType(5)),
		enc:        getEncoderByType(logEncoderType(6)),
	}

	config := zapcore.EncoderConfig{
		MessageKey:    "msg",
		LevelKey:      "lvl",
		EncodeLevel:   cc.(*commonCore).lvlEnc,
		TimeKey:       "ts",
		EncodeTime:    cc.(*commonCore).tsEnc,
		CallerKey:     "callAt",
		EncodeCaller:  zapcore.ShortCallerEncoder,
		FunctionKey:   "fn",
		NameKey:       "component",
		EncodeName:    zapcore.FullNameEncoder,
		StacktraceKey: coreKeyIgnored,
	}
	cc.(*commonCore).core = zapcore.NewCore(cc.(*commonCore).enc(config), cc.(*commonCore).ws, cc.(*commonCore).lvlEnabler)
	require.NotNil(t, cc.outEncoder())
	require.NotNil(t, cc.writeSyncer())
	require.NotNil(t, cc.levelEncoder())
	require.NotNil(t, cc.timeEncoder())
	require.NotNil(t, cc.(*commonCore).lvlEnabler)
	require.NotNil(t, cc.(*commonCore).core)

	require.True(t, cc.Enabled(zapcore.DebugLevel))
	require.True(t, cc.Enabled(zapcore.InfoLevel))
	require.True(t, cc.Enabled(zapcore.WarnLevel))
	require.True(t, cc.Enabled(zapcore.ErrorLevel))

	lvlEnabler.SetLevel(zapcore.ErrorLevel)
	require.False(t, cc.Enabled(zapcore.DebugLevel))
	require.False(t, cc.Enabled(zapcore.InfoLevel))
	require.False(t, cc.Enabled(zapcore.WarnLevel))
	require.True(t, cc.Enabled(zapcore.ErrorLevel))

	lvlEnabler.SetLevel(zapcore.DebugLevel)

	core := cc.With([]zap.Field{zap.String("key", "value")})
	require.NotNil(t, core)

	ent := cc.Check(zapcore.Entry{Level: zapcore.DebugLevel}, nil)
	err := cc.Write(ent.Entry, []zap.Field{zap.String("key", "value")})
	require.NoError(t, err)
	_ = cc.Sync()

	cc, err = WrapCore(cc, componentCoreEncoderCfg())
	require.NoError(t, err)
	require.NotNil(t, cc)
	err = cc.Write(zapcore.Entry{Level: zapcore.DebugLevel, LoggerName: "commonCore"}, []zap.Field{zap.String("key", "value")})
	require.NoError(t, err)
	_ = cc.Sync()
}
