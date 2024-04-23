package id

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/benz9527/xboot/lib/hrtime"
)

// go test -run none -bench BenchmarkStandardSnowflake_IDGen -benchtime 3s -benchmem
func BenchmarkStandardSnowflake_IDGen(b *testing.B) {
	hrtime.ClockInit()
	idGen, err := StandardSnowFlakeID(1, 1, func() time.Time {
		return hrtime.NowInDefaultTZ()
	})
	require.NoError(b, err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idGen()
	}
	b.ReportAllocs()
}

func TestStandardSnowflake_IDGen(t *testing.T) {
	hrtime.ClockInit()

	idGen, err := StandardSnowFlakeID(-1, 1, func() time.Time {
		return hrtime.NowInDefaultTZ()
	})
	require.Error(t, err)

	idGen, err = StandardSnowFlakeID(1, -1, func() time.Time {
		return hrtime.NowInDefaultTZ()
	})
	require.Error(t, err)

	idGen, err = StandardSnowFlakeID(1, 1, func() time.Time {
		return hrtime.NowInDefaultTZ()
	})
	require.NoError(t, err)
	id1 := idGen()
	id2 := idGen()
	require.NotEqual(t, id1, id2)
}

func TestSnowFlakeID(t *testing.T) {
	gen, err := SnowFlakeID(-1, 1, func() time.Time {
		return time.Now()
	})
	require.Error(t, err)

	gen, err = SnowFlakeID(1, -1, func() time.Time {
		return time.Now()
	})
	require.Error(t, err)

	gen, err = SnowFlakeID(1, 1, func() time.Time {
		return time.Now()
	})
	require.Nil(t, err)
	for i := 0; i < 1000; i++ {
		require.Less(t, gen.Number(), gen.Number())
	}
	require.NotEqual(t, gen.Str(), gen.Str())
}
