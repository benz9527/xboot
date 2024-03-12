package id

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMonotonicNonZeroID(t *testing.T) {
	gen, err := MonotonicNonZeroID()
	assert.Nil(t, err)
	for i := 0; i < 1000; i++ {
		t.Logf("%d, %s\n", gen.NumberUUID(), gen.StrUUID())
	}
}
