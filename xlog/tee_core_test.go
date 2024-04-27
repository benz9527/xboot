package xlog

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/benz9527/xboot/lib/id"
)

func TestConsoleAndFileMultiCores_DataRace(t *testing.T) {
	tee := make(xLogMultiCore, 0, 2)
	require.Nil(t, tee.context())
	require.Nil(t, tee.writeSyncer())
	require.Nil(t, tee.levelEncoder())
	require.Nil(t, tee.timeEncoder())
	require.Nil(t, tee.outEncoder())

	lvlEnabler := zap.NewAtomicLevelAt(LogLevelDebug.zapLevel())
	ctx, cancel := context.WithCancel(context.TODO())
	cc := newConsoleCore(
		ctx,
		&lvlEnabler,
		JSON,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)

	nano, err := id.ClassicNanoID(6)
	require.NoError(t, err)
	rngLogSuffix := "_" + nano() + "_xlog"

	cfg := &FileCoreConfig{
		FilePath: os.TempDir(),
		Filename: filepath.Base(os.Args[0]) + rngLogSuffix + ".log",
	}
	fc := newFileCore(cfg)(
		ctx,
		&lvlEnabler,
		JSON,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)
	tee1, err := WrapCores(nil, defaultCoreEncoderCfg())
	require.Error(t, err)
	require.Nil(t, tee1)

	tee1, err = WrapCores([]xLogCore{nil}, defaultCoreEncoderCfg())
	require.Error(t, err)
	require.Nil(t, tee1)

	tee1 = XLogTeeCore(cc, fc)
	require.Equal(t, zapcore.DebugLevel, tee1.(xLogMultiCore).Level())

	tee2, err := WrapCores(tee1.(xLogMultiCore), defaultCoreEncoderCfg())
	require.NoError(t, err)

	lvlEnabler2 := zap.NewAtomicLevelAt(LogLevelDebug.zapLevel())
	tee3, err := WrapCoresNewLevelEnabler(nil, &lvlEnabler2, defaultCoreEncoderCfg())
	require.Error(t, err)
	require.Nil(t, tee3)

	tee3, err = WrapCoresNewLevelEnabler([]xLogCore{nil}, &lvlEnabler2, defaultCoreEncoderCfg())
	require.Error(t, err)
	require.Nil(t, tee3)

	tee3, err = WrapCoresNewLevelEnabler(tee2.(xLogMultiCore), &lvlEnabler2, defaultCoreEncoderCfg())
	require.NoError(t, err)

	tee4 := tee3.(xLogMultiCore).With([]zap.Field{zap.String("fields", "tee4")})

	var ws sync.WaitGroup
	ws.Add(4)
	go func() {
		ent := tee1.Check(zapcore.Entry{Level: zapcore.DebugLevel}, nil)
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			if tee.Enabled(ent.Entry.Level) {
				err := tee1.Write(ent.Entry, []zap.Field{zap.String("tee1", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog tee write test!")})
				require.NoError(t, err)
			}
		}
		ws.Done()
	}()
	go func() {
		ent := tee2.Check(zapcore.Entry{Level: zapcore.InfoLevel}, nil)
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			if tee2.Enabled(ent.Entry.Level) {
				err := tee2.Write(ent.Entry, []zap.Field{zap.String("tee2", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog tee write test!")})
				require.NoError(t, err)
			}
		}
		ws.Done()
	}()
	go func() {
		ent := tee3.Check(zapcore.Entry{Level: zapcore.InfoLevel}, nil)
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			if tee3.Enabled(ent.Entry.Level) {
				err := tee3.Write(ent.Entry, []zap.Field{zap.String("tee3", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog tee write test!")})
				require.NoError(t, err)
			}
		}
		ws.Done()
	}()
	go func() {
		ent := tee4.Check(zapcore.Entry{Level: zapcore.WarnLevel}, nil)
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			if tee4.Enabled(ent.Entry.Level) {
				err := tee4.Write(ent.Entry, []zap.Field{zap.String("tee4", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog tee write test!")})
				require.NoError(t, err)
			}
		}
		ws.Done()
	}()
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = tee1.Sync()
		_ = tee2.Sync()
		_ = tee3.Sync()
		_ = tee4.Sync()
		t.Log("info level change")
		lvlEnabler.SetLevel(LogLevelInfo.zapLevel())

		time.Sleep(200 * time.Millisecond)
		_ = tee1.Sync()
		_ = tee2.Sync()
		_ = tee3.Sync()
		_ = tee4.Sync()
		t.Log("debug level change")
		lvlEnabler.SetLevel(LogLevelDebug.zapLevel())

		time.Sleep(300 * time.Millisecond)
		_ = tee1.Sync()
		_ = tee2.Sync()
		_ = tee3.Sync()
		_ = tee4.Sync()
		t.Log("warn level no other tee1 and tee2 logs")
		lvlEnabler.SetLevel(LogLevelWarn.zapLevel())

		time.Sleep(50 * time.Millisecond)
		_ = tee1.Sync()
		_ = tee2.Sync()
		_ = tee3.Sync()
		_ = tee4.Sync()
		t.Log("error level no other tee1, tee2 and tee3 logs")
		lvlEnabler2.SetLevel(LogLevelWarn.zapLevel())
	}()
	ws.Wait()

	_ = tee1.Sync()
	_ = tee2.Sync()
	_ = tee3.Sync()
	_ = tee4.Sync()
	cancel()

	removed := testCleanLogFiles(t, cfg.FilePath, filepath.Base(os.Args[0])+rngLogSuffix, ".log")
	require.Equal(t, 1, removed)
}
