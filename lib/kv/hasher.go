//go:build go1.22
// +build go1.22

package kv

import (
	randv2 "math/rand/v2"
	"unsafe"
)

type hashFn func(unsafe.Pointer, uintptr) uintptr

// Copy from go1.22.1
// go/src/internal/abi/type.go
type _mapType struct {
	_      [9]uint64                             // go/src/internal/abi/type.go Type: size 48, 6 bytes; key, elem, bucket: size 8 * 3, 3 bytes
	hasher func(unsafe.Pointer, uintptr) uintptr // function for hashing keys (ptr to key, seed) -> hash
	_      uint64                                // key size, value size, bucket size, flags
}

type _mapIface struct {
	typ *_mapType
	_   uint64 // go/src/runtime/map.go, hmap pointer, size 8, 1 byte
}

//go:nosplit
//go:nocheckptr
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

func newHashSeed() uintptr {
	return uintptr(randv2.Int())
}

type Hasher[K comparable] struct {
	hash hashFn
	seed uintptr
}

func (h Hasher[K]) Hash(key K) uint64 {
	// Promise the key no escapes to the heap.
	p := noescape(unsafe.Pointer(&key))
	return uint64(h.hash(p, h.seed))
}

func getRuntimeHasher[K comparable]() (fn hashFn) {
	i := (any)(make(map[K]struct{}))
	iface := (*_mapIface)(unsafe.Pointer(&i))
	fn = iface.typ.hasher
	return
}

func newHasher[K comparable]() Hasher[K] {
	return Hasher[K]{
		hash: getRuntimeHasher[K](),
		seed: newHashSeed(),
	}
}

func newSeedHasher[K comparable](hasher Hasher[K]) Hasher[K] {
	return Hasher[K]{
		hash: hasher.hash,
		seed: newHashSeed(),
	}
}
