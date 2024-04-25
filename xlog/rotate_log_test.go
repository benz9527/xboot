package xlog

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestParseFileAgeUnit(t *testing.T) {
	testcases := []struct {
		age         string
		expected    time.Duration
		expectedErr bool
	}{
		{
			"1s",
			1 * time.Second,
			false,
		},
		{
			"1sec",
			1 * time.Second,
			false,
		},
		{
			"1S",
			0,
			true,
		},
		{
			"_S",
			0,
			true,
		},
		{
			"_Sec",
			0,
			true,
		},
		{
			"1m",
			0,
			true,
		},
		{
			"1min",
			1 * time.Minute,
			false,
		},
		{
			"1H",
			1 * time.Hour,
			false,
		},
		{
			"1hour",
			1 * time.Hour,
			false,
		},
		{
			"2hours",
			2 * time.Hour,
			false,
		},
		{
			"2Hours",
			2 * time.Hour,
			false,
		},
		{
			"1D",
			1 * time.Duration(Day),
			false,
		},
		{
			"1d",
			1 * time.Duration(Day),
			false,
		},
		{
			"1day",
			1 * time.Duration(Day),
			false,
		},
		{
			"2days",
			2 * time.Duration(Day),
			false,
		},
		{
			"2Days",
			2 * time.Duration(Day),
			false,
		},
	}
	for _, tc := range testcases {
		actual, err := parseFileAge(tc.age)
		if tc.expectedErr {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, tc.expected, actual)
	}
}

func testRotateLogWriteRunCore(t *testing.T, log *RotateLog) {
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

func TestRotateLog_Write_Compress(t *testing.T) {
	log := &RotateLog{
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
		testRotateLogWriteRunCore(t, log)
	}
	reader, err := zip.OpenReader(filepath.Join(log.FilePath, log.FileZipName))
	require.NoError(t, err)
	require.LessOrEqual(t, int((loop-1)*log.FileMaxBackups), len(reader.File))
	reader.Close()
	removed := testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+"_xlog", ".log")
	require.Equal(t, log.FileMaxBackups+1, removed)
	removed = testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+"_xlogs", ".zip")
	require.Equal(t, 1, removed)
}

func TestRotateLog_Write_Delete(t *testing.T) {
	log := &RotateLog{
		FileMaxSize:      "1KB",
		Filename:         filepath.Base(os.Args[0]) + "_xlog.log",
		FileCompressible: false,
		FileMaxBackups:   4,
		FileMaxAge:       "3day",
		FilePath:         os.TempDir(),
	}
	loop := 2
	for i := 0; i < loop; i++ {
		testRotateLogWriteRunCore(t, log)
	}
	removed := testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+"_xlog", ".log")
	require.Equal(t, log.FileMaxBackups+1, removed)
}

func testCleanLogFiles(t *testing.T, path, namePrefix, nameSuffix string) int {
	// Walk through the log files and find the expired ones.
	entries, err := os.ReadDir(path)
	logInfos := make([]os.FileInfo, 0, 16)
	if err == nil && len(entries) > 0 {
		for _, entry := range entries {
			if !entry.IsDir() {
				filename := entry.Name()
				if strings.HasPrefix(filename, namePrefix) && strings.HasSuffix(filename, nameSuffix) {
					if info, err := entry.Info(); err == nil && info != nil {
						logInfos = append(logInfos, info)
					}
				}
			}
		}
	}
	for _, logInfo := range logInfos {
		err = os.Remove(filepath.Join(path, logInfo.Name()))
		require.NoError(t, err)
	}
	return len(logInfos)
}
