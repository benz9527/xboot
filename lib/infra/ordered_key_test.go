package infra

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComplexCompare(t *testing.T) {
	var c1 complex128 = complex(1.0, 2.0) // 1.0+2.0i
	var c2 complex128 = complex(1.1, 2.0) // 1.1+2.0i
	_c1 := math.Hypot(real(c1), imag(c1))
	_c2 := math.Hypot(real(c2), imag(c2))
	assert.Greater(t, _c2, _c1)
}
