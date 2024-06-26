package list

// References:
// https://gitee.com/bombel/cdf_skiplist
// http://snap.stanford.edu/data/index.html
//
// Goland math random (math.Float64()) contains global mutex lock
// Ref
// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/rand/rand.go
// https://cs.opensource.google/go/go/+/refs/tags/go1.21.5:src/math/bits/bits.go
// 1. Avoid using global mutex lock
// 2. Avoid generating random number each time

import (
	crand "crypto/rand"
	"encoding/binary"
	"math"
	"math/bits"
	randv2 "math/rand/v2"
)

type SklRand func(maxLevel int, currentElements int64) int32

func maxLevels(totalElements int64, P float64) int {
	// Ref https://www.cl.cam.ac.uk/teaching/2005/Algorithms/skiplists.pdf
	// maxLevels = log(1/P) * log(totalElements)
	// P = 1/4, totalElements = 2^32 - 1
	if totalElements <= 0 {
		return 0
	}
	return int(math.Ceil(math.Log(1/P) * math.Log(float64(totalElements))))
}

func randomLevel(maxLevel int, currentElements int32) int32 {
	level := 1
	for float64(randv2.Int64()&0xFFFF) < sklProbability*0xFFFF {
		level += 1
	}
	if level < sklMaxLevel {
		return int32(level)
	}
	return sklMaxLevel
}

// randomLevelV2 is the skip list level element.
// Dynamic level calculation.
func randomLevelV2(maxLevel int, currentElements int64) int32 {
	// Call function maxLevels to get total?
	// maxLevel => n, 2^n-1, there will be 2^n-1 elements in the skip list
	var total uint64
	if maxLevel == sklMaxLevel {
		total = uint64(math.MaxUint32)
	} else {
		total = uint64(1)<<maxLevel - 1
	}
	// Bits right shift equal to manipulate a high-level bit
	// Calculate the minimum bits of the random number
	// Lookup table.
	level := maxLevel - bits.Len64(randv2.Uint64()&total) + 1
	// Avoid the value of randomly generated level deviates
	// far from the number of elements within the skip-list.
	// Level should be greater than but approximate to log(currentElements)
	for level > 3 && uint64(1)<<(level-3) > uint64(currentElements) {
		level--
	}
	return int32(level)
}

// randomLevelV3 is the skip list level element.
// Dynamic level calculation.
// Concurrency safe.
func randomLevelV3(maxLevel int, currentElements int64) int32 {
	// Call function maxLevels to get total?
	// maxLevel => n, 2^n-1, there will be 2^n-1 elements in the skip list
	var total uint64
	if maxLevel == sklMaxLevel {
		total = uint64(math.MaxUint32)
	} else {
		total = uint64(1)<<maxLevel - 1
	}

	num := cryptoRandUint64()
	rest := num & total
	// Bits right shift equal to manipulate a high-level bit
	// Calculate the minimum bits of the random number
	tmp := bits.Len64(rest) // Lookup table.
	level := maxLevel - tmp + 1
	// Avoid the value of randomly generated level deviates
	// far from the number of elements within the skip-list.
	// level should be greater than but approximate to log(currentElements)
	for level > 3 && uint64(1)<<(level-3) > uint64(currentElements) {
		level--
	}
	return int32(level)
}

func cryptoRandUint64() uint64 {
	randUint64 := [8]byte{}
	if _, err := crand.Read(randUint64[:]); err != nil {
		panic(err)
	}
	if randUint64[7]&0x8 == 0x0 {
		return binary.LittleEndian.Uint64(randUint64[:])
	}
	return binary.BigEndian.Uint64(randUint64[:])
}

func cryptoRandUint32() uint32 {
	randUint32 := [4]byte{}
	if _, err := crand.Read(randUint32[:]); err != nil {
		panic(err)
	}
	if randUint32[3]&0x8 == 0x0 {
		return binary.LittleEndian.Uint32(randUint32[:])
	}
	return binary.BigEndian.Uint32(randUint32[:])
}
