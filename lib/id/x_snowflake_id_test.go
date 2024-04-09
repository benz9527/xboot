package id

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestXSnowflakeID_53BitsMask(t *testing.T) {
	require.Equal(t, int64(0b111), -1^(int64(-1)<<3))
	require.Equal(t, int64(0x1F_FFFF_FFFF_FFFF), xSnowflakeTsAndSequenceMask)
}

func TestGetMACAsWorkerID(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Skip("unable to fetch interfaces MAC", err)
	}

	iface := interfaces[0]
	if len(iface.HardwareAddr) <= 0 {
		t.Skip("unable to fetch interfaces MAC", iface)
	}

	t.Log(iface.HardwareAddr[4], iface.HardwareAddr[5])
}

func TestXSnowFlakeID_ByMAC(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Skip("unable to fetch interfaces MAC", err)
	}

	iface := interfaces[0]
	if len(iface.HardwareAddr) <= 0 {
		t.Skip("unable to fetch interfaces MAC", iface)
	}

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
		require.Less(t, gen.Number(), gen.Number())
	}
}

func TestXSnowFlakeID_ByMAC_DataRace(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Skip("unable to fetch interfaces MAC", err)
	}

	iface := interfaces[0]
	if len(iface.HardwareAddr) <= 0 {
		t.Skip("unable to fetch interfaces MAC", iface)
	}
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
		go func() {
			require.Less(t, gen.Number(), gen.Number())
		}()
	}
}

func BenchmarkXSnowFlakeID_ByMAC(b *testing.B) {
	interfaces, err := net.Interfaces()
	if err != nil {
		b.Skip("unable to fetch interfaces MAC", err)
	}

	iface := interfaces[0]
	if len(iface.HardwareAddr) <= 0 {
		b.Skip("unable to fetch interfaces MAC", iface)
	}

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
