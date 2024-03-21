package id

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMonotonicNonZeroID(t *testing.T) {
	gen, err := MonotonicNonZeroID()
	assert.Nil(t, err)
	for i := 0; i < 1000; i++ {
		t.Logf("%d, %s\n", gen.Number(), gen.Str())
	}
}
