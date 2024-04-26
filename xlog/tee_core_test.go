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
	tee = append(tee, cc)

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
	tee = append(tee, fc)

	tee2, err := WrapCores(tee, defaultCoreEncoderCfg())
	require.NoError(t, err)

	var ws sync.WaitGroup
	ws.Add(2)
	go func() {
		ent := cc.Check(zapcore.Entry{Level: zapcore.DebugLevel}, nil)
		for i := 0; i < 100; i++ {
			time.Sleep(time.Millisecond * 5)
			err := tee.Write(ent.Entry, []zap.Field{zap.String("tee", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog tee write test!")})
			require.NoError(t, err)
		}
		ws.Done()
	}()
	go func() {
		ent := cc.Check(zapcore.Entry{Level: zapcore.InfoLevel}, nil)
		for i := 0; i < 100; i++ {
			time.Sleep(time.Millisecond * 5)
			err := tee2.Write(ent.Entry, []zap.Field{zap.String("tee2", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog tee write test!")})
			require.NoError(t, err)
		}
		ws.Done()
	}()
	go func() {
		time.Sleep(100 * time.Millisecond)
		t.Log("info level change")
		err = tee.Sync()
		require.NoError(t, err)
		err = tee2.Sync()
		require.NoError(t, err)
		lvlEnabler.SetLevel(LogLevelInfo.zapLevel())
		time.Sleep(100 * time.Millisecond)
		t.Log("debug level change")
		err = tee.Sync()
		require.NoError(t, err)
		err = tee2.Sync()
		require.NoError(t, err)
		lvlEnabler.SetLevel(LogLevelDebug.zapLevel())
		time.Sleep(200 * time.Millisecond)
		t.Log("warn level no other logs")
		err = tee.Sync()
		require.NoError(t, err)
		err = tee2.Sync()
		require.NoError(t, err)
		lvlEnabler.SetLevel(LogLevelWarn.zapLevel())
	}()
	ws.Wait()

	err = tee.Sync()
	require.NoError(t, err)
	err = tee2.Sync()
	require.NoError(t, err)
	cancel()

	removed := testCleanLogFiles(t, cfg.FilePath, filepath.Base(os.Args[0])+"_xlog", ".log")
	require.Equal(t, 1, removed)
}
