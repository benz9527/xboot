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

	cfg := &FileCoreConfig{
		FilePath: os.TempDir(),
		Filename: filepath.Base(os.Args[0]) + "_xlog.log",
	}
	fc := newFileCore(cfg)(
		ctx,
		&lvlEnabler,
		JSON,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)
	tee1 := XLogTeeCore(cc, fc)

	tee2, err := WrapCores(tee1.(xLogMultiCore), defaultCoreEncoderCfg())
	require.NoError(t, err)

	lvlEnabler2 := zap.NewAtomicLevelAt(LogLevelDebug.zapLevel())
	tee3, err := WrapCoresNewLevelEnabler(tee2.(xLogMultiCore), &lvlEnabler2, defaultCoreEncoderCfg())
	require.NoError(t, err)

	var ws sync.WaitGroup
	ws.Add(3)
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
		time.Sleep(200 * time.Millisecond)
		_ = tee1.Sync()
		_ = tee2.Sync()
		t.Log("info level change")
		lvlEnabler.SetLevel(LogLevelInfo.zapLevel())

		time.Sleep(200 * time.Millisecond)
		_ = tee1.Sync()
		_ = tee2.Sync()
		t.Log("debug level change")
		lvlEnabler.SetLevel(LogLevelDebug.zapLevel())

		time.Sleep(300 * time.Millisecond)
		_ = tee1.Sync()
		_ = tee2.Sync()
		t.Log("warn level no other tee1 and tee2 logs")
		lvlEnabler.SetLevel(LogLevelWarn.zapLevel())
	}()
	ws.Wait()

	_ = tee1.Sync()
	_ = tee2.Sync()
	cancel()

	removed := testCleanLogFiles(t, cfg.FilePath, filepath.Base(os.Args[0])+"_xlog", ".log")
	require.Equal(t, 1, removed)
}
