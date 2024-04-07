package kv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestThreadSafeMap_ListKeys(t *testing.T) {
	keys := genStrKeys(8, 10000)
	vals := make([]int, 0, len(keys))
	m := make(map[string]int, len(keys))
	_m := NewThreadSafeMap[string, int]()
	for i, key := range keys {
		m[key] = i
		vals = append(vals, i)
	}
	_m.Replace(m)

	_keys := _m.ListKeys()
	require.Equal(t, len(keys), len(_keys))
	require.ElementsMatch(t, keys, _keys)

	_vals := _m.ListValues()
	require.ElementsMatch(t, vals, _vals)

	i := 1001
	res, exists := _m.Get(keys[i])
	require.True(t, exists)
	require.Equal(t, i, res)

	res, err := _m.Delete(keys[i])
	require.NoError(t, err)
	require.Equal(t, i, res)

	err = _m.AddOrUpdate(keys[i], i)
	require.NoError(t, err)

	_keys = _m.ListKeys()
	require.Equal(t, len(keys), len(_keys))
	require.ElementsMatch(t, keys, _keys)

	_vals = _m.ListValues()
	require.ElementsMatch(t, vals, _vals)

	err = _m.Purge()
	require.NoError(t, err)
}
