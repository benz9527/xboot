package id

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetMACAsWorkerID(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Skip("unable to fetch interfaces MAC", err)
	}

	iface := interfaces[0]
	t.Log(iface.HardwareAddr[4], iface.HardwareAddr[5])
}

func TestXSnowFlakeID_ByMAC(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Skip("unable to fetch interfaces MAC", err)
	}

	iface := interfaces[0]
	workerID := ((int64(iface.HardwareAddr[4]) & 0b11) << 8) | int64(iface.HardwareAddr[5]&0xFF)
	gen, err := XSnowFlakeIDByWorkerID(
		workerID,
		func() time.Time {
			return time.Now()
		},
	)
	require.NoError(t, err)
	require.NotNil(t, gen)
	for i := 0; i < 1000; i++ {
		t.Logf("%d, %s\n", gen.Number(), gen.Str())
	}
}

func BenchmarkXSnowFlakeID_ByMAC(b *testing.B) {
	interfaces, err := net.Interfaces()
	if err != nil {
		b.Skip("unable to fetch interfaces MAC", err)
	}

	iface := interfaces[0]
	workerID := ((int64(iface.HardwareAddr[4]) & 0b11) << 8) | int64(iface.HardwareAddr[5]&0xFF)
	gen, err := XSnowFlakeIDByWorkerID(
		workerID,
		func() time.Time {
			return time.Now()
		},
	)
	require.NoError(b, err)
	require.NotNil(b, gen)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gen.Number()
	}
	b.ReportAllocs()
}
