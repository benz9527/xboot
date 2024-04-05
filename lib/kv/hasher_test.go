package kv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasher(t *testing.T) {
	intKey := 100
	intHasher := newHasher[int]()
	require.Equal(t, intHasher.Hash(intKey), intHasher.Hash(intKey))

	strKey := "abc"
	strHasher := newHasher[string]()
	require.Equal(t, strHasher.Hash(strKey), strHasher.Hash(strKey))

	floatKey := 100.0
	floatHasher := newHasher[float64]()
	require.Equal(t, floatHasher.Hash(floatKey), floatHasher.Hash(floatKey))
}
