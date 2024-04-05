//go:build !amd64 || nosimd
// +build !amd64 nosimd

package kv

// It is inconvenient to use SIMD on non-AMD64 platforms.
// https://graphics.stanford.edu/~seander/bithacks.html##ValueInWord
// bit manipulation is inconvenient for [16]int8{}, there
// is not exists uint128 in Go.
// Otherwise, we can check the hash value like:
// (0x0101_0101_0101_0101_0101_0101_0101_0101 * hash) ^ uint128([16]int8)
// Then convert it to uint16 by byte manipulation.
func Fast16WayHashMatch(md *[16]int8, hash int8) uint16 {
	res := uint16(0)
	for i := 0; i < 16; i++ {
		if md[i] == hash { // XOR byte.
			res |= 1 << uint(i)
		}
	}
	return res
}
