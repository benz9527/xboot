package kv

import (
	"math/bits"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrailingZeros16(t *testing.T) {
	bitset := uint16(0x0001)
	for i := 0; i < 16; i++ {
		tmp := bitset << i
		require.Equal(t, i, bits.TrailingZeros16(tmp))
	}
}
