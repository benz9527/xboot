package xlog

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/benz9527/xboot/lib/id"
)

func TestXLogFileCore_RotateLog(t *testing.T) {
	nano, err := id.ClassicNanoID(6)
	require.NoError(t, err)
	rngLogSuffix := "_" + nano() + "_xlog"
	rngLogZipSuffix := rngLogSuffix + "s"
	lvlEnabler := zap.NewAtomicLevelAt(LogLevelDebug.zapLevel())
	cfg := &FileCoreConfig{
		FilePath:                os.TempDir(),
		Filename:                filepath.Base(os.Args[0]) + rngLogSuffix + ".log",
		FileCompressible:        true,
		FileMaxBackups:          4,
		FileMaxAge:              "3day",
		FileMaxSize:             "1KB",
		FileCompressBatch:       2,
		FileZipName:             filepath.Base(os.Args[0]) + rngLogZipSuffix + ".zip",
		FileRotateEnable:        true,
		FileBufferSize:          "1KB",
		FileBufferFlushInterval: 500,
	}

	ctx, cancel := context.WithCancel(context.TODO())
	cc := newFileCore(cfg)(
		nil,
		&lvlEnabler,
		JSON,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)
	require.Nil(t, cc)

	cc = newFileCore(cfg)(
		ctx,
		&lvlEnabler,
		JSON,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)

	ent := cc.Check(zapcore.Entry{Level: zapcore.DebugLevel}, nil)
	for i := 0; i < 100; i++ {
		err := cc.Write(ent.Entry, []zap.Field{zap.String("key", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog rolling log write test!")})
		require.NoError(t, err)
	}
	time.Sleep(1 * time.Second)
	err = cc.Sync()
	require.NoError(t, err)
	cancel()

	reader, err := zip.OpenReader(filepath.Join(cfg.FilePath, cfg.FileZipName))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(reader.File), cfg.FileCompressBatch)
	err = reader.Close()
	require.NoError(t, err)
	
	removed := testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+rngLogSuffix, ".log")
	require.GreaterOrEqual(t, removed, cfg.FileMaxBackups+1)
	removed = testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+rngLogZipSuffix, ".zip")
	require.Equal(t, removed, 1)
}

func TestXLogFileCore_SingleLog(t *testing.T) {
	nano, err := id.ClassicNanoID(6)
	require.NoError(t, err)
	rngLogSuffix := "_" + nano() + "_xlog"
	rngLogZipSuffix := rngLogSuffix + "s"
	lvlEnabler := zap.NewAtomicLevelAt(LogLevelDebug.zapLevel())
	cfg := &FileCoreConfig{
		FilePath: os.TempDir(),
		Filename: filepath.Base(os.Args[0]) + rngLogSuffix + ".log",
	}

	ctx, cancel := context.WithCancel(context.TODO())
	cc := newFileCore(cfg)(
		nil,
		&lvlEnabler,
		JSON,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)
	require.Nil(t, cc)

	cc = newFileCore(cfg)(
		ctx,
		&lvlEnabler,
		JSON,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)

	ent := cc.Check(zapcore.Entry{Level: zapcore.DebugLevel}, nil)
	for i := 0; i < 100; i++ {
		err := cc.Write(ent.Entry, []zap.Field{zap.String("key", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog single log write test!")})
		require.NoError(t, err)
	}
	time.Sleep(1 * time.Second)
	err = cc.Sync()
	require.NoError(t, err)
	cancel()

	removed := testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+rngLogSuffix, ".log")
	require.GreaterOrEqual(t, removed, cfg.FileMaxBackups+1)
	removed = testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+rngLogZipSuffix, ".zip")
	require.Equal(t, 0, removed)
}

func TestXLogFileCore_RotateLog_DataRace(t *testing.T) {
	nano, err := id.ClassicNanoID(6)
	require.NoError(t, err)
	rngLogSuffix := "_" + nano() + "_xlog"
	rngLogZipSuffix := rngLogSuffix + "s"
	lvlEnabler := zap.NewAtomicLevelAt(LogLevelDebug.zapLevel())
	cfg := &FileCoreConfig{
		FilePath:                os.TempDir(),
		Filename:                filepath.Base(os.Args[0]) + rngLogSuffix + ".log",
		FileCompressible:        true,
		FileMaxBackups:          4,
		FileMaxAge:              "3day",
		FileMaxSize:             "1KB",
		FileCompressBatch:       2,
		FileZipName:             filepath.Base(os.Args[0]) + rngLogZipSuffix + ".zip",
		FileRotateEnable:        true,
		FileBufferSize:          "1KB",
		FileBufferFlushInterval: 500,
	}

	ctx, cancel := context.WithCancel(context.TODO())
	cc := newFileCore(cfg)(
		nil,
		&lvlEnabler,
		JSON,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)
	require.Nil(t, cc)

	cc = newFileCore(cfg)(
		ctx,
		&lvlEnabler,
		JSON,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)

	go func() {
		ent := cc.Check(zapcore.Entry{Level: zapcore.DebugLevel}, nil)
		for i := 0; i < 100; i++ {
			err := cc.Write(ent.Entry, []zap.Field{zap.String("key", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog rolling log write test!")})
			require.NoError(t, err)
		}
	}()
	go func() {
		ent := cc.Check(zapcore.Entry{Level: zapcore.DebugLevel}, nil)
		for i := 100; i < 200; i++ {
			err := cc.Write(ent.Entry, []zap.Field{zap.String("key", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog rolling log write test!")})
			require.NoError(t, err)
		}
	}()

	time.Sleep(1 * time.Second)
	err = cc.Sync()
	require.NoError(t, err)
	cancel()

	reader, err := zip.OpenReader(filepath.Join(cfg.FilePath, cfg.FileZipName))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(reader.File), cfg.FileCompressBatch)
	err = reader.Close()
	require.NoError(t, err)
	removed := testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+rngLogSuffix, ".log")
	require.GreaterOrEqual(t, removed, cfg.FileMaxBackups+1)
	removed = testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+rngLogZipSuffix, ".zip")
	require.Equal(t, removed, 1)
}

func TestXLogFileCore_SingleLog_DataRace(t *testing.T) {
	nano, err := id.ClassicNanoID(6)
	require.NoError(t, err)
	rngLogSuffix := "_" + nano() + "_xlog"
	lvlEnabler := zap.NewAtomicLevelAt(LogLevelDebug.zapLevel())
	cfg := &FileCoreConfig{
		FilePath: os.TempDir(),
		Filename: filepath.Base(os.Args[0]) + rngLogSuffix + ".log",
	}

	ctx, cancel := context.WithCancel(context.TODO())
	cc := newFileCore(cfg)(
		nil,
		&lvlEnabler,
		JSON,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)
	require.Nil(t, cc)

	cc = newFileCore(cfg)(
		ctx,
		&lvlEnabler,
		JSON,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)

	go func() {
		ent := cc.Check(zapcore.Entry{Level: zapcore.DebugLevel}, nil)
		for i := 0; i < 100; i++ {
			err := cc.Write(ent.Entry, []zap.Field{zap.String("key", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog single log write test!")})
			require.NoError(t, err)
		}
	}()
	go func() {
		ent := cc.Check(zapcore.Entry{Level: zapcore.DebugLevel}, nil)
		for i := 100; i < 200; i++ {
			err := cc.Write(ent.Entry, []zap.Field{zap.String("key", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog single log write test!")})
			require.NoError(t, err)
		}
	}()

	time.Sleep(1 * time.Second)
	err = cc.Sync()
	require.NoError(t, err)
	cancel()

	removed := testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+rngLogSuffix, ".log")
	require.GreaterOrEqual(t, removed, 1)
}
