package xlog

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
