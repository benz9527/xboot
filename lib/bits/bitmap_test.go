package bits

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewX32Bitmap(t *testing.T) {
	bm := NewX32Bitmap(10)
	bm2 := NewX32Bitmap(10)
	originalOffsets := []uint64{9, 5, 7, 3, 2, 8, 1}
	expectedOffsets := []uint64{1, 2, 3, 5, 7, 8, 9}
	for _, offset := range originalOffsets {
		bm.SetBit(offset)
		bm2.SetBit(offset)
	}
	bm.SetBit(100)
	bm.UnsetBit(100)
	bm2.UnsetBit(4)
	for _, offset := range expectedOffsets {
		require.True(t, bm.GetBit(offset))
	}
	require.True(t, bm.EqualTo(bm2))
	require.False(t, bm.GetBit(100))
	bm.Purge()

	bm = NewX32Bitmap(maxBitMapSize + 1)
	bm.Purge()
}
