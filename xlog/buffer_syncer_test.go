package xlog

import (
	"testing"
	"time"

	"github.com/benz9527/xboot/lib/id"
	"github.com/stretchr/testify/require"
)

type outWriter struct {
	data [][]byte
}

func (w *outWriter) Write(data []byte) (n int, err error) {
	l := len(data)
	tmp := make([]byte, l)
	copy(tmp, data)
	w.data = append(w.data, tmp)
	return l, nil
}

func (w *outWriter) Close() error {
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
	w := &outWriter{}
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
