package id

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMonotonicNonZeroID(t *testing.T) {
	gen, err := MonotonicNonZeroID()
	require.Nil(t, err)
	require.Equal(t, uint64(1), gen.Number())
	for i := 0; i < 1000; i++ {
		require.Less(t, gen.Number(), gen.Number())
	}
}

func BenchmarkMonotonicNonZeroID(b *testing.B) {
	gen, err := MonotonicNonZeroID()
	require.Nil(b, err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gen.Number()
	}
	b.ReportAllocs()
}
