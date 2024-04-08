package id

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonotonicNonZeroID(t *testing.T) {
	gen, err := MonotonicNonZeroID()
	assert.Nil(t, err)
	for i := 0; i < 1000; i++ {
		require.Less(t, gen.Number(), gen.Number())
	}
}
