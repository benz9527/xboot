package xlog

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestXLogFileCore_RotateLog(t *testing.T) {
	lvlEnabler := zap.NewAtomicLevelAt(LogLevelDebug.zapLevel())
	cfg := &FileCoreConfig{
		FilePath:                os.TempDir(),
		Filename:                filepath.Base(os.Args[0]) + "_xlog.log",
		FileCompressible:        true,
		FileMaxBackups:          4,
		FileMaxAge:              "3day",
		FileMaxSize:             "1KB",
		FileCompressBatch:       2,
		FileZipName:             filepath.Base(os.Args[0]) + "_xlogs.zip",
		FileRotateEnable:        true,
		FileBufferSize:          "1KB",
		FileBufferFlushInterval: 500,
	}

	cc := newFileCore(cfg)(
		&lvlEnabler,
		JSON,
		testMemAsOut,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)
	require.Nil(t, cc)

	cc = newFileCore(cfg)(
		&lvlEnabler,
		JSON,
		File,
		zapcore.CapitalLevelEncoder,
		zapcore.ISO8601TimeEncoder,
	)

	ent := cc.Check(zapcore.Entry{Level: zapcore.DebugLevel}, nil)
	for i := 0; i < 100; i++ {
		err := cc.Write(ent.Entry, []zap.Field{zap.String("key", strconv.Itoa(i)+" "+time.Now().UTC().Format(backupDateTimeFormat)+" xlog rolling log write test!")})
		require.NoError(t, err)
	}
	time.Sleep(1 * time.Second)
	err := cc.Sync()
	require.NoError(t, err)

	reader, err := zip.OpenReader(filepath.Join(cfg.FilePath, cfg.FileZipName))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(reader.File), cfg.FileCompressBatch)
	err = reader.Close()
	require.NoError(t, err)
	// testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+"_xlog", ".log")
	// testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+"_xlogs", ".zip")
}
