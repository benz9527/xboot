//go:build amd64 && !nosimd
// +build amd64,!nosimd

package kv

import "math/bits"

// SSE2:
// Streaming SIMD Extensions 2 is one of the Intel SIMD (single instruction, multiple data)
// processor supplementary instruction sets introduced by Intel with the initial version 
// of the Pentium 4 in 2000.
//
// SSSE3:
// Supplemental Streaming SIMD Extensions 3 (SSSE3).
//
// AVX:
// Advanced Vector Extensions.

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
