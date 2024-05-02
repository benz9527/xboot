package xlog

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/safeopen"
	"github.com/stretchr/testify/require"

	"github.com/benz9527/xboot/lib/id"
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

func testRotateLogWriteRunCore(t *testing.T, log *rotateLog) {
	err := log.initialize()
	require.NoError(t, err)

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
	nano, err := id.ClassicNanoID(6)
	require.NoError(t, err)
	rngLogSuffix := "_" + nano() + "_xlog"
	rngLogZipSuffix := rngLogSuffix + "s"
	log := &rotateLog{
		fileMaxSize:       "1KB",
		filename:          filepath.Base(os.Args[0]) + rngLogSuffix + ".log",
		fileCompressible:  true,
		fileMaxBackups:    4,
		fileMaxAge:        "3day",
		fileCompressBatch: 2,
		fileZipName:       filepath.Base(os.Args[0]) + rngLogZipSuffix + ".zip",
		filePath:          os.TempDir(),
		ctx:               context.TODO(),
	}
	loop := 2
	for i := 0; i < loop; i++ {
		testRotateLogWriteRunCore(t, log)
	}
	reader, err := zip.OpenReader(filepath.Join(log.filePath, log.fileZipName))
	require.NoError(t, err)
	require.LessOrEqual(t, int((loop-1)*log.fileMaxBackups), len(reader.File))
	reader.Close()
	removed := testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+rngLogSuffix, ".log")
	require.LessOrEqual(t, log.fileMaxBackups+1, removed)
	removed = testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+rngLogZipSuffix, ".zip")
	require.Equal(t, 1, removed)
}

func TestRotateLog_Write_Delete(t *testing.T) {
	nano, err := id.ClassicNanoID(6)
	require.NoError(t, err)
	rngLogSuffix := "_" + nano() + "_xlog"
	log := &rotateLog{
		fileMaxSize:      "1KB",
		filename:         filepath.Base(os.Args[0]) + rngLogSuffix + ".log",
		fileCompressible: false,
		fileMaxBackups:   4,
		fileMaxAge:       "3day",
		filePath:         os.TempDir(),
		ctx:              context.TODO(),
	}
	loop := 2
	for i := 0; i < loop; i++ {
		testRotateLogWriteRunCore(t, log)
	}
	removed := testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+rngLogSuffix, ".log")
	require.Equal(t, log.fileMaxBackups+1, removed)
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
			} else {
				if entry.Name() == namePrefix+nameSuffix {
					if info, err := entry.Info(); err == nil && info != nil {
						logInfos = append(logInfos, info)
					}
				}
			}
		}
	}
	for _, logInfo := range logInfos {
		_ = os.Remove(filepath.Join(path, logInfo.Name()))
	}
	return len(logInfos)
}

func TestRotateLog_Write_PermissionDeniedAccess(t *testing.T) {
	rf, err := safeopen.CreateBeneath(os.TempDir(), "rpda.log")
	require.NoError(t, err)
	err = rf.Close()
	require.NoError(t, err)

	err = os.Chmod(filepath.Join(os.TempDir(), "rpda.log"), 0o400)
	require.NoError(t, err)

	rf, err = safeopen.OpenFileBeneath(os.TempDir(), "rpda.log", os.O_WRONLY|os.O_APPEND, 0o666)
	require.Error(t, err) // Access denied.
	require.Nil(t, rf)

	ctx, cancel := context.WithCancel(context.TODO())
	log := RotateLog(nil, &FileCoreConfig{})
	require.Nil(t, log)
	log = RotateLog(ctx, nil)
	require.Nil(t, log)

	log = RotateLog(ctx, &FileCoreConfig{
		FileMaxSize:      "1KB",
		Filename:         "rpda.log",
		FilePath:         os.TempDir(),
		FileCompressible: false,
		FileMaxAge:       "100days",
		FileMaxBackups:   4,
	})

	_, err = log.Write([]byte("rotate log permission denied access!"))
	require.Error(t, err) // Access denied.
	cancel()
	err = log.Close()
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	_, err = log.Write([]byte("rotate log permission denied access!"))
	require.True(t, errors.Is(err, io.EOF))

	removed := testCleanLogFiles(t, os.TempDir(), "rpda", ".log")
	require.Equal(t, 1, removed)
}

func TestRotateLog_Write_Dir(t *testing.T) {
	err := os.Mkdir(filepath.Join(os.TempDir(), "rpda2.log"), 0o600)
	require.NoError(t, err)

	log := &rotateLog{
		fileMaxSize:      "1KB",
		filename:         "rpda2.log",
		fileCompressible: false,
		fileMaxBackups:   4,
		fileMaxAge:       "3day",
		filePath:         os.TempDir(),
		ctx:              context.TODO(),
	}

	_, err = log.Write([]byte("rotate log write dir!"))
	require.Error(t, err)
	err = log.Close()
	require.NoError(t, err)

	removed := testCleanLogFiles(t, os.TempDir(), "rpda2", ".log")
	require.Equal(t, 1, removed)
}

func TestRotateLog_Write_OtherErrors(t *testing.T) {
	log := &rotateLog{
		fileMaxSize:      "1KB",
		filename:         "rpda3.log",
		fileCompressible: false,
		fileMaxBackups:   4,
		fileMaxAge:       "3day",
		filePath:         os.TempDir(),
		ctx:              context.TODO(),
	}

	err := log.openOrCreate()
	require.NoError(t, err)
	err = log.Close()
	require.NoError(t, err)

	log.filePath = "abc"
	err = log.openOrCreate()
	require.Error(t, err)

	removed := testCleanLogFiles(t, os.TempDir(), "rpda3", ".log")
	require.Equal(t, 1, removed)
}
