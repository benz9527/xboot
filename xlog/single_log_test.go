package xlog

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSingleLog(t *testing.T) {
	closeC := make(chan struct{})
	log := SingleLog(nil, closeC)
	require.Nil(t, log)

	log = SingleLog(&FileCoreConfig{
		FilePath: os.TempDir(),
		Filename: filepath.Base(os.Args[0]) + "_sxlog.log",
	}, nil)
	require.Nil(t, log)

	log = SingleLog(&FileCoreConfig{
		FilePath: os.TempDir(),
		Filename: filepath.Base(os.Args[0]) + "_sxlog.log",
	}, closeC)

	for i := 0; i < 1000; i++ {
		data := []byte(strconv.Itoa(i) + " " + time.Now().UTC().Format(backupDateTimeFormat) + " xlog single log write test!\n")
		_, err := log.Write(data)
		require.NoError(t, err)
	}
	err := log.Close()
	require.NoError(t, err)

	close(closeC)
	err = log.Close()
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	_, err = log.Write([]byte("xlog single log write test!\n"))
	require.True(t, errors.Is(err, io.EOF))

	log = &singleLog{
		filename: filepath.Base(os.Args[0]) + "_sxlog.log",
	}

	for i := 2000; i < 3000; i++ {
		data := []byte(strconv.Itoa(i) + " " + time.Now().UTC().Format(backupDateTimeFormat) + " xlog single log write test!\n")
		_, err = log.Write(data)
		require.NoError(t, err)
	}
	err = log.Close()
	require.NoError(t, err)

	removed := testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+"_sxlog", ".log")
	require.Equal(t, 1, removed)
}

func TestSingleLog_PermissionDeniedAccess(t *testing.T) {
	rf, err := os.Create(filepath.Join(os.TempDir(), "pda.log"))
	require.NoError(t, err)
	err = rf.Close()
	require.NoError(t, err)

	err = os.Chmod(filepath.Join(os.TempDir(), "pda.log"), 0o400)
	require.NoError(t, err)

	rf, err = os.OpenFile(filepath.Join(os.TempDir(), "pda.log"), os.O_WRONLY|os.O_APPEND, 0o666)
	require.Error(t, err)
	require.True(t, os.IsPermission(err))
	require.Nil(t, rf)

	log := &singleLog{
		filename: "pda.log",
		filePath: os.TempDir(),
		closeC:   make(chan struct{}),
	}
	_, err = log.Write([]byte("permission denied access!"))
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrPermission))
	err = log.Close()
	require.NoError(t, err)

	removed := testCleanLogFiles(t, os.TempDir(), "pda", ".log")
	require.Equal(t, 1, removed)
}

func TestSingleLog_Write_Dir(t *testing.T) {
	err := os.Mkdir(filepath.Join(os.TempDir(), "pda2.log"), 0o600)
	require.NoError(t, err)

	log := &singleLog{
		filename: "pda2.log",
		filePath: os.TempDir(),
		closeC:   make(chan struct{}),
	}

	_, err = log.Write([]byte("single log write dir!"))
	require.Error(t, err)
	err = log.Close()
	require.NoError(t, err)

	removed := testCleanLogFiles(t, os.TempDir(), "pda2", ".log")
	require.Equal(t, 1, removed)
}
