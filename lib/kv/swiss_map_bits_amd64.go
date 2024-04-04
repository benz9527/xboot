//go:build amd64 && !nosimd
// +build amd64,!nosimd

package kv

import "math/bits"

const (
	groupSize       = 16
	maxAvgGroupLoad = 14
)

func metadataMatchH2(md *swissMapMetadata, h2 h2) bitset {
	b := MatchMetadata((*[16]int8)(md), int8(h2))
	return bitset(b)
}

func metadataMatchEmpty(md *swissMapMetadata) bitset {
	b := MatchMetadata((*[16]int8)(md), empty)
	return bitset(b)
}

func nextMatch(bs *bitset) uint32 {
	s := uint32(bits.TrailingZeros16(uint16(*bs)))
	*bs &= ^(1 << s) // unset bits
	return s
}
