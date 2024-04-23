package xlog

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"
)

func TestParseFileSizeUnit(t *testing.T) {
	testcases := []struct {
		size        string
		expected    uint64
		expectedErr bool
	}{
		{
			"abcMB",
			0,
			true,
		},
		{
			"_GB",
			0,
			true,
		},
		{
			"TB",
			0,
			true,
		},
		{
			"Y",
			0,
			true,
		},
		{
			"100B",
			100 * uint64(B),
			false,
		},
		{
			"100KB",
			100 * uint64(KB),
			false,
		},
		{
			"100MB",
			100 * uint64(MB),
			false,
		},
		{
			"100b",
			100 * uint64(B),
			false,
		},
		{
			"100kb",
			100 * uint64(KB),
			false,
		},
		{
			"100mb",
			100 * uint64(MB),
			false,
		},
		{
			"100kB",
			100 * uint64(KB),
			false,
		},
		{
			"100Mb",
			100 * uint64(MB),
			false,
		},
		{
			"100Kb",
			100 * uint64(KB),
			false,
		},
		{
			"100mB",
			100 * uint64(MB),
			false,
		},
	}
	for _, tc := range testcases {
		actual, err := parseFileSize(tc.size)
		if tc.expectedErr {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, tc.expected, actual)
	}
}

func testRollingLogWriteRunCore(t *testing.T, log *RollingLog) {
	size, err := parseFileSize(log.FileMaxSize)
	require.NoError(t, err)
	require.Equal(t, uint64(1024), size)
	log.maxSize = size
	log.fileWatcher, err = fsnotify.NewWatcher()
	log.fileWatcher.Add(log.FilePath)
	require.NoError(t, err)
	go log.watchAndArchive()

	for i := 0; i < 100; i++ {
		data := []byte(strconv.Itoa(i) + " " + time.Now().UTC().Format(backupDateTimeFormat) + " xlog rolling log write test!\n")
		_, err = log.Write(data)
		require.NoError(t, err)
	}
	time.Sleep(1 * time.Second)
	err = log.Close()
	require.NoError(t, err)
}

func TestRollingLog_Write(t *testing.T) {
	log := &RollingLog{
		FileMaxSize:       "1KB",
		Filename:          filepath.Base(os.Args[0]) + "_xlog.log",
		FileCompressible:  true,
		FileMaxBackups:    4,
		FileMaxAge:        "3day",
		FileCompressBatch: 2,
		FileZipName:       filepath.Base(os.Args[0]) + "_xlogs.zip",
		FilePath:          os.TempDir(),
	}
	loop := 2
	for i := 0; i < loop; i++ {
		testRollingLogWriteRunCore(t, log)
	}
	reader, err := zip.OpenReader(filepath.Join(log.FilePath, log.FileZipName))
	require.NoError(t, err)
	require.LessOrEqual(t, int((loop-1)*log.FileMaxBackups), len(reader.File))
}
