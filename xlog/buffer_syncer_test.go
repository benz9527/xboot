package xlog

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/benz9527/xboot/lib/id"
)

type syncerOutWriter struct {
	data [][]byte
}

func (w *syncerOutWriter) Write(data []byte) (n int, err error) {
	l := len(data)
	tmp := make([]byte, l)
	copy(tmp, data)
	w.data = append(w.data, tmp)
	return l, nil
}

func (w *syncerOutWriter) Close() error {
	return nil
}

func genLog(strLen, count int) (keys []string) {
	nanoID, err := id.ClassicNanoID(strLen)
	if err != nil {
		panic(err)
	}
	keys = make([]string, count)
	for i := range keys {
		keys[i] = nanoID()
	}
	return
}

func TestXLogBufferSyncer_Console(t *testing.T) {
	w := &syncerOutWriter{}
	syncer := &XLogBufferSyncer{
		outWriter: w,
		arena: &xLogArena{
			size: 1 << 10,
		},
		flushInterval: 500 * time.Millisecond,
	}
	syncer.initialize()

	logs := genLog(100, 200)
	for _, log := range logs {
		_, err := syncer.Write([]byte(log))
		require.NoError(t, err)
	}
	time.Sleep(1 * time.Second)
	err := syncer.Sync()
	require.NoError(t, err)
	require.NotZero(t, len(w.data))
	for i, log := range logs {
		require.Equal(t, w.data[i], []byte(log))
	}
	syncer.Stop()
}

func TestXLogBufferSyncer_Console_DataRace(t *testing.T) {
	w := &syncerOutWriter{}
	syncer := &XLogBufferSyncer{
		outWriter: w,
		arena: &xLogArena{
			size: 1 << 10,
		},
		flushInterval: 500 * time.Millisecond,
	}
	syncer.initialize()

	wg := sync.WaitGroup{}
	wg.Add(2)
	logs := genLog(100, 200)
	go func() {
		for i := 0; i < len(logs)>>1; i++ {
			_, err := syncer.Write([]byte(logs[i]))
			require.NoError(t, err)
		}
		wg.Done()
	}()
	go func() {
		for i := len(logs) >> 1; i < len(logs); i++ {
			_, err := syncer.Write([]byte(logs[i]))
			require.NoError(t, err)
		}
		wg.Done()
	}()
	wg.Wait()
	time.Sleep(1 * time.Second)
	err := syncer.Sync()
	require.NoError(t, err)
	require.NotZero(t, len(w.data))
	set := make(map[string]struct{}, len(logs))
	for _, log := range logs {
		set[log] = struct{}{}
	}
	for _, log := range w.data {
		_, ok := set[string(log)]
		require.True(t, ok)
	}
	syncer.Stop()
}

func testBufferSyncerRollingLogWriteRunCore(t *testing.T, log *RollingLog, syncer zapcore.WriteSyncer) {
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
		_, err = syncer.Write(data)
		require.NoError(t, err)
	}
	time.Sleep(1 * time.Second)
	err = log.Close()
	require.NoError(t, err)
}

func testBufferSyncerRollingLogWriteDataRaceRunCore(t *testing.T, log *RollingLog, syncer zapcore.WriteSyncer) {
	size, err := parseFileSize(log.FileMaxSize)
	require.NoError(t, err)
	require.Equal(t, uint64(1024), size)
	log.maxSize = size
	log.fileWatcher, err = fsnotify.NewWatcher()
	log.fileWatcher.Add(log.FilePath)
	require.NoError(t, err)
	go log.watchAndArchive()
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		for i := 0; i < 50; i++ {
			data := []byte(strconv.Itoa(i) + " " + time.Now().UTC().Format(backupDateTimeFormat) + " xlog rolling log write test!\n")
			_, err = syncer.Write(data)
			require.NoError(t, err)
		}
		wg.Done()
	}()
	go func() {
		for i := 50; i < 100; i++ {
			data := []byte(strconv.Itoa(i) + " " + time.Now().UTC().Format(backupDateTimeFormat) + " xlog rolling log write test!\n")
			_, err = syncer.Write(data)
			require.NoError(t, err)
		}
		wg.Done()
	}()
	wg.Wait()
	time.Sleep(1 * time.Second)
	err = log.Close()
	require.NoError(t, err)
}

func TestXLogBufferSyncer_RollingLog(t *testing.T) {
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

	syncer := &XLogBufferSyncer{
		outWriter: log,
		arena: &xLogArena{
			size: 1 << 10,
		},
		flushInterval: 500 * time.Millisecond,
	}
	syncer.initialize()

	loop := 2
	for i := 0; i < loop; i++ {
		testBufferSyncerRollingLogWriteRunCore(t, log, syncer)
	}
	reader, err := zip.OpenReader(filepath.Join(log.FilePath, log.FileZipName))
	require.NoError(t, err)
	require.LessOrEqual(t, int((loop-1)*log.FileMaxBackups), len(reader.File))
	reader.Close()
	testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+"_xlog", ".log")
	testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+"_xlogs", ".zip")
}

func TestXLogBufferSyncer_RollingLog_DataRace(t *testing.T) {
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

	syncer := &XLogBufferSyncer{
		outWriter: log,
		arena: &xLogArena{
			size: 1 << 10,
		},
		flushInterval: 500 * time.Millisecond,
	}
	syncer.initialize()

	loop := 2
	for i := 0; i < loop; i++ {
		testBufferSyncerRollingLogWriteDataRaceRunCore(t, log, syncer)
	}
	reader, err := zip.OpenReader(filepath.Join(log.FilePath, log.FileZipName))
	require.NoError(t, err)
	require.LessOrEqual(t, int((loop-1)*log.FileMaxBackups), len(reader.File))
	reader.Close()
	testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+"_xlog", ".log")
	testCleanLogFiles(t, os.TempDir(), filepath.Base(os.Args[0])+"_xlogs", ".zip")
}
