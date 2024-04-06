package id

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNanoID(t *testing.T) {
	nanoID, err := ClassicNanoID(8)
	require.NoError(t, err)
	for i := 0; i < 1000; i++ {
		t.Log(nanoID())
	}
}

func BenchmarkNanoID(b *testing.B) {
	nanoID, err := ClassicNanoID(8)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = nanoID()
	}
	b.ReportAllocs()
}
