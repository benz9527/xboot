package id

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/benz9527/xboot/lib/hrtime"
)

// go test -run none -bench Benchmark_standard_snow_flake_id_gen -benchtime 3s -benchmem
func Benchmark_standard_snow_flake_id_gen(b *testing.B) {
	asserter := assert.New(b)

	idGen, err := StandardSnowFlakeID(1, 1, func() time.Time {
		return hrtime.NowInDefaultTZ()
	})
	asserter.NoError(err)
	for i := 0; i < b.N; i++ {
		_ = idGen()
	}
}

func Test_standard_snow_flake_id_gen(t *testing.T) {
	asserter := assert.New(t)

	idGen, err := StandardSnowFlakeID(1, 1, func() time.Time {
		return hrtime.NowInDefaultTZ()
	})
	asserter.NoError(err)
	id1 := idGen()
	id2 := idGen()
	asserter.NotEqual(id1, id2)
	t.Logf("%d, %d", id1, id2)
}

func TestSnowFlakeID(t *testing.T) {
	gen, err := SnowFlakeID(1, 1, func() time.Time {
		return time.Now()
	})
	assert.Nil(t, err)
	for i := 0; i < 1000; i++ {
		require.Less(t, gen.Number(), gen.Number())
	}
}
