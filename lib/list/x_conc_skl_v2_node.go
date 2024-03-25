package list

import (
	"sync/atomic"
	"unsafe"

	ibits "github.com/benz9527/xboot/lib/bits"
	"github.com/benz9527/xboot/lib/infra"
)

// xConcSklV2Node, total size is 168 (42 bytes)
// @field key, type is infra.OrderedKey.
// unsafe pointer size 8, 2 bytes
// @field root, type is *xNode[V], it stores the
// (unique/duplicate) value.
// unsafe pointer size 8, 2 bytes
// @field mu, type is segmentMutex
// unsafe pointer size 8, 2 bytes
// @field indices, it is arena offset array.
// size 128, 32 bytes
type xConcSklV2Node[K infra.OrderedKey, V any] struct {
	_key     unsafe.Pointer
	_root    unsafe.Pointer
	_mu      unsafe.Pointer
	indices  [sklMaxLevel]uint32
	flagBits uint64
	count    uint32
	level    uint32
}

func (node *xConcSklV2Node[K, V]) set(bits uint64) {
	node.flagBits = node.flagBits | bits
}

func (node *xConcSklV2Node[K, V]) isSet(bit uint64) bool {
	return (node.flagBits & bit) != 0
}

func (node *xConcSklV2Node[K, V]) setBitsAs(bits, target uint64) {
	if ibits.HammingWeightBySWARV2[uint64](bits) <= 1 {
		panic("it is not a multi-bits")
	}
	n := 0
	for (bits>>n)&0x1 != 0x1 {
		n++
	}
	if n > 0 {
		target <<= n
	}
	check := node.flagBits & bits
	node.flagBits = node.flagBits ^ check
	node.flagBits = node.flagBits | target
}

func (node *xConcSklV2Node[K, V]) loadBits(bits uint64) uint64 {
	if ibits.HammingWeightBySWARV2[uint64](bits) <= 1 {
		panic("it is not a multi-bits")
	}
	n := 0
	for (bits>>n)&0x1 != 0x1 {
		n++
	}
	res := node.flagBits & bits
	if n > 0 {
		res >>= n
	}
	return res
}

func (node *xConcSklV2Node[K, V]) areEqual(bits, expect uint64) bool {
	return node.loadBits(bits) == expect
}

func (node *xConcSklV2Node[K, V]) atomicSet(bits uint64) {
	for {
		old := atomic.LoadUint64(&node.flagBits)
		if old&bits != bits {
			n := old | bits
			if atomic.CompareAndSwapUint64(&node.flagBits, old, n) {
				return
			}
			continue
		}
		return
	}
}

// Bit flag set from 1 to 0.
func (node *xConcSklV2Node[K, V]) atomicUnset(bits uint64) {
	for {
		old := atomic.LoadUint64(&node.flagBits)
		check := old & bits
		if check != 0 {
			n := old ^ check
			if atomic.CompareAndSwapUint64(&node.flagBits, old, n) {
				return
			}
			continue
		}
		return
	}
}

func (node *xConcSklV2Node[K, V]) atomicIsSet(bit uint64) bool {
	return (atomic.LoadUint64(&node.flagBits) & bit) != 0
}

func (node *xConcSklV2Node[K, V]) atomicAreEqual(bits, expect uint64) bool {
	if ibits.HammingWeightBySWARV2[uint64](bits) <= 1 {
		panic("it is not a multi-bits")
	}
	n := 0
	for (bits>>n)&0x1 != 0x1 {
		n++
	}
	if n > 0 {
		expect <<= n
	}
	return (atomic.LoadUint64(&node.flagBits) & bits) == expect
}

func (node *xConcSklV2Node[K, V]) atomicLoadBits(bits uint64) uint64 {
	if ibits.HammingWeightBySWARV2[uint64](bits) <= 1 {
		panic("it is not a multi-bits")
	}
	n := 0
	for (bits>>n)&0x1 != 0x1 {
		n++
	}
	res := atomic.LoadUint64(&node.flagBits) & bits
	if n > 0 {
		res >>= n
	}
	return res
}

func (node *xConcSklV2Node[K, V]) atomicSetBitsAs(bits, target uint64) {
	if ibits.HammingWeightBySWARV2[uint64](bits) <= 1 {
		panic("it is not a multi-bits")
	}
	n := 0
	for (bits>>n)&0x1 != 0x1 {
		n++
	}
	if n > 0 {
		target <<= n
	}

	for {
		old := atomic.LoadUint64(&node.flagBits)
		check := old & bits
		if check != 0 {
			n := old ^ check
			n = n | target
			if atomic.CompareAndSwapUint64(&node.flagBits, old, n) {
				return
			}
			continue
		}
		return
	}
}

func (node *xConcSklV2Node[K, V]) key() K {
	return *(*K)(node._key)
}

func (node *xConcSklV2Node[K, V]) root() *xNode[V] {
	return *(**xNode[V])(node._root)
}

func (node *xConcSklV2Node[K, V]) mu() segmentMutex {
	return *(*segmentMutex)(node._mu)
}
