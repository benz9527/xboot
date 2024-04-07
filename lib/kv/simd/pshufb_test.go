package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPSHUFB(t *testing.T) {
	var md [16]int8
	hash := int8(0b01100110)
	pshufb(&md, hash)
	for i := 0; i < 16; i++ {
		require.Equal(t, md[i], int8(0b01100110))
	}
}
