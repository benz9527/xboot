package kv

import (
	"errors"
	"math/bits"
	randv2 "math/rand/v2"
	"sync/atomic"

	ibits "github.com/benz9527/xboot/lib/bits"
)

// References:
// https://github.com/CppCon/CppCon2017
// https://www.dolthub.com/blog/2023-03-28-swiss-map/
// https://github.com/dolthub/swiss/blob/main/map.go
// https://github.com/thepudds/swisstable/blob/main/map.go
// https://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction/
// https://faultlore.com/blah/hashbrown-tldr/
// https://rcoh.me/posts/hash-map-analysis/
// https://github.com/abseil/abseil-cpp/blob/master/absl/container/flat_hash_map.h
// https://github.com/rust-lang/hashbrown
// https://blog.waffles.space/2018/12/07/deep-dive-into-hashbrown/#fn:4
// https://methane.hatenablog.jp/entry/2022/02/22/Swisstable_Hash_%E3%81%AB%E4%BD%BF%E3%82%8F%E3%82%8C%E3%81%A6%E3%81%84%E3%82%8B%E3%83%93%E3%83%83%E3%83%88%E6%BC%94%E7%AE%97%E3%81%AE%E9%AD%94%E8%A1%93
// https://www.youtube.com/watch?v=JZE3_0qvrMg
// https://github.com/abseil/abseil-cpp/blob/master/absl/container/internal/raw_hash_set.h

// Swiss Table, called Flat Hash Map also.
// Hash slot mapped to the key-val pair slot.
// Short hash from lo bits (1 byte) is an optimization to
// accelerate the hash lookup.
// SSE2 instruction the best performance for linear-probing is
// 16! (https://www.youtube.com/watch?v=ncHmEUmJZf4&t=1449s)

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

/*
 index |   0    |   1    |   2    |   3    |   4    | ... |   15   |
-------|--------|--------|--------|--------|--------|     |--------|
 value | (5,7)  |        | (39,8) |        |        | ... |        |
-------|--------|--------|--------|--------|--------|     |--------|
 ctrl  |01010111|11111111|00110110|11111111|11111111| ... |11111111|

1. hash map
It uses arrays as its backend. In the context of hash map, the array
elements are called buckets or slots. Keeps the key and value at the
same time, it is in order to decrease the hash collision.

2. load factor
It is the ratio of the number of elements in the hash map to the number
of buckets. Once we reach a certain load factor (like 0.5, 0.7 or 0.9)
hash map should resize and rehash all the key-value pairs.

3. optimization
Whenever the CPU needs to read/write to a memory location, it checks the
caches, and if it's present, it's a cache hit, otherwise it's a cache
missing. Whenever a cache miss occurs, we pay the cost of fetching the
data from main memory (thereby losing a few hundred CPU cycles by waiting).
The second way is to get rid of using external data structures completely,
and use the same array for storing values alongside buckets.

4. hash collision solution
open-addressing, traversing the array linearly. It is cache-friendly.
It can save CPU instruction cycles.

5. key-value deletion
5.1 In addition to remove a pair, we also move the next pair to that
slot (shift backwards).
5.2 Or we add a special flag (tombstone) to removed slots, and when
we probe, we can skip a slot containing that flag. But, this will have
bad effect on the load factor and very easily to trigger resize and
rehash.
5.3 robin hood hashing
In robin hood hashing, you follow one rule - if the distance to the
actual slot of the current element in the slot is less than the
distance to the actual slot of the element to be inserted, then we
swap both the elements and proceed.
*/

//go:generate go run ./simd/asm.go -out fast_hash_match.s -stubs fast_hash_match_amd64.go

const (
	slotSize              = 16 // In order to finding the results in 4 CPU instructions
	maxAvgSlotLoad        = 14
	h1Mask         uint64 = 0xffff_ffff_ffff_ff80
	h2Mask         uint64 = 0x0000_0000_0000_007f
	empty          int8   = -128 // 0b1000_0000, 0x80; https://github.com/abseil/abseil-cpp/blob/61e47a454c81eb07147b0315485f476513cc1230/absl/container/internal/raw_hash_set.h#L505
	deleted        int8   = -2   // 0b1111_1110, OxFE; https://github.com/abseil/abseil-cpp/blob/61e47a454c81eb07147b0315485f476513cc1230/absl/container/internal/raw_hash_set.h#L506
)

var (
	// amd64 && !nosimd 256 * 1024 * 1024; !amd64 || nosimd 512 * 1024 * 1024
	maxSlotCap = 1 << (32 - ibits.CeilPowOf2(slotSize))
)

// A 57 bits hash prefix.
// The whole hash truncated to a unsigned 64-bit integer.
// Used as an index into the groups array.
type h1 uint64

// A 7 bits hash suffix.
// The top 7 bits of the hash. In FULL control byte format.
type h2 int8

type bitset uint16

type swissMapMetadata [slotSize]int8

func (md *swissMapMetadata) matchH2(hash h2) bitset {
	b := Fast16WayHashMatch((*[slotSize]int8)(md), int8(hash))
	return bitset(b)
}

func (md *swissMapMetadata) matchEmpty() bitset {
	b := Fast16WayHashMatch((*[slotSize]int8)(md), empty)
	return bitset(b)
}

// Array is cache friendly.
type swissMapSlot[K comparable, V any] struct {
	keys [slotSize]K
	vals [slotSize]V
}

type SwissMap[K comparable, V any] struct {
	ctrlMetadataSet []swissMapMetadata
	slots           []swissMapSlot[K, V]
	hasher          Hasher[K]
	resident        uint64 // current alive elements
	dead            uint64 // current tombstone elements
	limit           uint64 // max resident elements
	slotCap         uint32
}

func (m *SwissMap[K, V]) Put(key K, val V) error {
	if m.resident >= m.limit {
		n, err := m.nextCap()
		if err != nil {
			return err
		}
		if err = m.rehash(n); err != nil {
			return err
		}
	}
	m.put(key, val)
	return nil
}

func (m *SwissMap[K, V]) put(key K, val V) {
	h1, h2 := splitHash(m.hasher.Hash(key))
	i := findSlotIndex(h1, atomic.LoadUint32(&m.slotCap))
	for {
		for result := m.ctrlMetadataSet[i].matchH2(h2); /* exists */ result != 0; {
			if /* hash collision */ j := nextIndexInSlot(&result);
			/* key equal, update */ key == m.slots[i].keys[j] {
				m.slots[i].keys[j] = key
				m.slots[i].vals[j] = val
				return
			}
		}

		if /* not found */ result := m.ctrlMetadataSet[i].matchEmpty(); /* insert */ result != 0 {
			n := nextIndexInSlot(&result)
			m.slots[i].keys[n] = key
			m.slots[i].vals[n] = val
			m.ctrlMetadataSet[i][n] = int8(h2)
			m.resident++
			return
		}
		if /* open-addressing (linear-probing) */ i += 1; /* close loop */ i >= atomic.LoadUint32(&m.slotCap) {
			i = 0
		}
	}
}

func (m *SwissMap[K, V]) Get(key K) (val V, exists bool) {
	h1, h2 := splitHash(m.hasher.Hash(key))
	i := findSlotIndex(h1, atomic.LoadUint32(&m.slotCap))
	for {
		for result := m.ctrlMetadataSet[i].matchH2(h2); /* exists */ result != 0; {
			if /* hash collision */ j := nextIndexInSlot(&result); /* found */ key == m.slots[i].keys[j] {
				return m.slots[i].vals[j], true
			}
		}
		if /* not found */ m.ctrlMetadataSet[i].matchEmpty() != 0 {
			return val, false
		}
		if /* open-addressing (linear-probing) */ i += 1; /* close loop */ i >= atomic.LoadUint32(&m.slotCap) {
			i = 0
		}
	}
}

func (m *SwissMap[K, V]) Foreach(action func(i uint64, key K, val V) bool) {
	oldCtrlMetadataSet, oldSlots, oldSlotCap := m.ctrlMetadataSet, m.slots, atomic.LoadUint32(&m.slotCap)
	rngIdx := randv2.Uint32N(oldSlotCap)
	idx := uint64(0)
	for i := uint32(0); i < oldSlotCap; i++ {
		for j, md := range oldCtrlMetadataSet[rngIdx] {
			if md == empty || md == deleted {
				continue
			}
			k, v := oldSlots[rngIdx].keys[j], oldSlots[rngIdx].vals[j]
			if _continue := action(idx, k, v); !_continue {
				return
			}
			idx++
		}
		if /* open-addressing (linear-probing) */ rngIdx += 1; /* close loop */ rngIdx >= oldSlotCap {
			rngIdx = 0
		}
	}
}

func (m *SwissMap[K, V]) Delete(key K) (val V, err error) {
	h1, h2 := splitHash(m.hasher.Hash(key))
	i := findSlotIndex(h1, atomic.LoadUint32(&m.slotCap))
	for {
		for result := m.ctrlMetadataSet[i].matchH2(h2); /* exists */ result != 0; {
			if /* hash collision */ j := nextIndexInSlot(&result); /* found */ key == m.slots[i].keys[j] {
				val = m.slots[i].vals[j]

				if m.ctrlMetadataSet[i].matchEmpty() > 0 {
					// SIMD 16-way hash match result is start from the trailing.
					// The empty control byte in trailing will not cause premature
					// termination of linear-probing.
					// In order to terminate the deletion linear-probing quickly.
					m.ctrlMetadataSet[i][j] = empty
					m.resident--
				} else {
					m.ctrlMetadataSet[i][j] = deleted
					m.dead++
				}

				var (
					k K
					v V
				)
				m.slots[i].keys[j] = k
				m.slots[i].vals[j] = v
				return
			}
		}
		if /* not found */ m.ctrlMetadataSet[i].matchEmpty() != 0 {
			// Found the most likely slot index at first.
			// So if the key not in the slot, it should be
			// store in next slot. If next slot contains
			// empty control byte before h2 linear-probing,
			// it means that key not exists.
			return val, errors.New("[swiss-map] not found to delete")
		}

		if /* open-addressing (linear-probing) */ i += 1; /* close loop */ i >= atomic.LoadUint32(&m.slotCap) {
			i = 0
		}
	}
}

func (m *SwissMap[K, V]) Clear() {
	var (
		k K
		v V
	)
	for i := uint32(0); i < atomic.LoadUint32(&m.slotCap); i++ {
		slot := &m.slots[i]
		for j := 0; j < slotSize; j++ {
			m.ctrlMetadataSet[i][j] = empty
			slot.keys[j] = k
			slot.vals[j] = v
		}
	}
	m.resident, m.dead = 0, 0
}

func (m *SwissMap[K, V]) MigrateFrom(_m map[K]V) error {
	for k, v := range _m {
		if err := m.Put(k, v); err != nil {
			return err
		}
	}
	return nil
}

func (m *SwissMap[K, V]) Len() int64 {
	return int64(m.resident - m.dead)
}

func (m *SwissMap[K, V]) Cap() int64 {
	return int64(m.limit - m.resident)
}

func (m *SwissMap[K, V]) nextCap() (uint32, error) {
	if m.dead >= (m.resident >> 1) {
		return atomic.LoadUint32(&m.slotCap), nil
	}
	newCap := int64(atomic.LoadUint32(&m.slotCap)) * 2
	if newCap > int64(maxSlotCap) {
		return 0, errors.New("[swiss-map] slots overflow")
	}
	return uint32(newCap), nil
}

func (m *SwissMap[K, V]) rehash(newCapacity uint32) error {
	oldCtrlMetadataSet, oldSlots, oldSlotCap := m.ctrlMetadataSet, m.slots, atomic.LoadUint32(&m.slotCap)
	if !atomic.CompareAndSwapUint32(&m.slotCap, oldSlotCap, newCapacity) {
		return errors.New("[swiss-map] concurrent rehash")
	}

	m.slots = make([]swissMapSlot[K, V], newCapacity)
	m.ctrlMetadataSet = make([]swissMapMetadata, newCapacity)
	for i := uint32(0); i < atomic.LoadUint32(&m.slotCap); i++ {
		m.ctrlMetadataSet[i] = newEmptyMetadata()
	}

	m.hasher = newSeedHasher[K](m.hasher)
	m.limit = uint64(newCapacity) * maxAvgSlotLoad
	m.resident, m.dead = 0, 0
	for i := uint32(0); i < oldSlotCap; i++ {
		for j := 0; j < slotSize; j++ {
			if md := oldCtrlMetadataSet[i][j]; md == empty || md == deleted {
				continue
			}
			m.put(oldSlots[i].keys[j], oldSlots[i].vals[j])
		}
	}
	return nil
}

func (m *SwissMap[K, V]) loadFactor() float64 {
	total := float64(atomic.LoadUint32(&m.slotCap) * slotSize)
	return float64(m.resident-m.dead) / total
}

// @param size, how many elements will be stored in the map
func NewSwissMap[K comparable, V any](capacity uint32) *SwissMap[K, V] {
	slotCap := calcSlotCapacity(capacity)
	m := &SwissMap[K, V]{
		ctrlMetadataSet: make([]swissMapMetadata, slotCap),
		slots:           make([]swissMapSlot[K, V], slotCap),
		slotCap:         slotCap,
		hasher:          newHasher[K](),
		resident:        0,
		dead:            0,
		limit:           uint64(slotCap) * maxAvgSlotLoad,
	}
	for i := uint32(0); i < slotCap; i++ {
		m.ctrlMetadataSet[i] = newEmptyMetadata()
	}
	return m
}

func calcSlotCapacity(size uint32) uint32 {
	groupCap := (size + maxAvgSlotLoad - 1) / maxAvgSlotLoad
	if groupCap == 0 {
		groupCap = 1
	}
	return groupCap
}

func newEmptyMetadata() swissMapMetadata {
	var m swissMapMetadata
	for i := 0; i < slotSize; i++ {
		m[i] = empty
	}
	return m
}

func splitHash(hash uint64) (hi h1, lo h2) {
	return h1((hash & h1Mask) >> 7), h2(hash & h2Mask)
}

// Check which slot that the key will be placed.
func findSlotIndex(hi h1, groups uint32) uint32 {
	return uint32(hi) & (groups - 1)
}

// Hash collision, find bit as index, start from the trailing then unset it.
func nextIndexInSlot(bs *bitset) uint32 {
	trail := uint32(bits.TrailingZeros16(uint16(*bs)))
	*bs &= ^(1 << trail)
	return trail
}
