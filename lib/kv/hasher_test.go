package kv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type hashObj struct {
	val1 int
	val2 string
	val3 bool
	val4 *float64
	val5 map[string]struct{}
	val6 []int
}

func TestHasher(t *testing.T) {
	intKey := 100
	intHasher := newHasher[int]()
	require.Equal(t, intHasher.Hash(intKey), intHasher.Hash(intKey))

	intPtrHasher := newHasher[*int]()
	require.Equal(t, intPtrHasher.Hash(&intKey), intPtrHasher.Hash(&intKey))

	strKey := "abc"
	strHasher := newHasher[string]()
	require.Equal(t, strHasher.Hash(strKey), strHasher.Hash(strKey))

	strPtrHasher := newHasher[*string]()
	require.Equal(t, strPtrHasher.Hash(&strKey), strPtrHasher.Hash(&strKey))

	floatKey := 100.0
	floatHasher := newHasher[float64]()
	require.Equal(t, floatHasher.Hash(floatKey), floatHasher.Hash(floatKey))

	floatPtrHasher := newHasher[*float64]()
	require.Equal(t, floatPtrHasher.Hash(&floatKey), floatPtrHasher.Hash(&floatKey))

	objKey := hashObj{val1: 100, val2: "abc", val3: true, val4: nil, val5: nil, val6: nil}
	objPtrHasher := newHasher[*hashObj]()
	require.Equal(t, objPtrHasher.Hash(&objKey), objPtrHasher.Hash(&objKey))

	newSeedObjPtrHasher := newSeedHasher[*hashObj](objPtrHasher)
	require.NotEqual(t, objPtrHasher.Hash(&objKey), newSeedObjPtrHasher.Hash(&objKey))
}
