package xlog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestConsoleCore(t *testing.T) {
	lvlEnabler := zap.NewAtomicLevelAt(LogLevelDebug.zapLevel())
	var cc XLogCore = newConsoleCore(
		context.TODO(),
		&lvlEnabler,
		JSON,
		testMemAsOut,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)
	require.Nil(t, cc)

	cc = newConsoleCore(
		context.TODO(),
		&lvlEnabler,
		JSON,
		StdOut,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)
	require.NotNil(t, cc.outEncoder())
	require.NotNil(t, cc.writeSyncer())
	require.NotNil(t, cc.levelEncoder())
	require.NotNil(t, cc.timeEncoder())
	require.NotNil(t, cc.(*consoleCore).core.lvlEnabler)
	require.NotNil(t, cc.(*consoleCore).core.core)

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

	cc, err = WrapCore(cc, componentCoreEncoderCfg)
	require.NoError(t, err)
	require.NotNil(t, cc)
	err = cc.Write(zapcore.Entry{Level: zapcore.DebugLevel, LoggerName: "commonCore"}, []zap.Field{zap.String("key", "value")})
	require.NoError(t, err)
	_ = cc.Sync()
}
