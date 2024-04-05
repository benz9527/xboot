package kv

import (
	"errors"
	"math"
	randv2 "math/rand/v2"

	"github.com/benz9527/xboot/lib/infra"
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
// http://graphics.stanford.edu/~seander/bithacks.html##ValueInWord
// https://methane.hatenablog.jp/entry/2022/02/22/Swisstable_Hash_%E3%81%AB%E4%BD%BF%E3%82%8F%E3%82%8C%E3%81%A6%E3%81%84%E3%82%8B%E3%83%93%E3%83%83%E3%83%88%E6%BC%94%E7%AE%97%E3%81%AE%E9%AD%94%E8%A1%93
// https://www.youtube.com/watch?v=JZE3_0qvrMg
// https://github.com/abseil/abseil-cpp/blob/master/absl/container/internal/raw_hash_set.h

// Swiss Table, called Flat Hash Map also.
// Hash slot mapped to the key-val pair slot.
// Short hash from lo bits (1 byte) is an optimization to
// accelerate the hash lookup.
// SSE2 instruction the best performance for linear-probing is
// 16! (https://www.youtube.com/watch?v=ncHmEUmJZf4&t=1449s)

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

//go:generate go run ./simd/asm.go -out match_metadata.s -stubs match_metadata_amd64.go

const (
	h1Mask  uint64 = 0xffff_ffff_ffff_ff80
	h2Mask  uint64 = 0x0000_0000_0000_007f
	empty   int8   = -128 // 0b1000_0000, 0x80; https://github.com/abseil/abseil-cpp/blob/61e47a454c81eb07147b0315485f476513cc1230/absl/container/internal/raw_hash_set.h#L505
	deleted int8   = -2   // 0b1111_1110, OxFE; https://github.com/abseil/abseil-cpp/blob/61e47a454c81eb07147b0315485f476513cc1230/absl/container/internal/raw_hash_set.h#L506
)

// A 57 bits hash prefix.
// The whole hash truncated to a unsigned 64-bit integer.
// Used as an index into the groups array.
type h1 uint64

// A 7 bits hash suffix.
// The top 7 bits of the hash. In FULL control byte format.
type h2 int8

type bitset uint16

type swissMapMetadata [groupSize]int8

// Array is cache friendly.
type swissMapGroup[K infra.OrderedKey, V any] struct {
	keys [groupSize]K
	vals [groupSize]V
}

type SwissMap[K infra.OrderedKey, V any] struct {
	ctrlMetadatas []swissMapMetadata
	groups       []swissMapGroup[K, V]
	hasher       Hasher[K]
	resident     uint32 // current alive elements
	dead         uint32 // current tombstone elements
	limit        uint32 // max resident elements
}

func (m *SwissMap[K, V]) Put(key K, val V) error {
	if m.resident >= m.limit {
		n, err := m.nextCap()
		if err != nil {
			return err
		}
		m.rehash(n)
	}
	m.put(key, val)
	return nil
}

func (m *SwissMap[K, V]) put(key K, val V) {
	h1, h2 := splitHash(m.hasher.Hash(key))
	// Check which group that the key will be placed.
	i := linearProbing(h1, uint32(len(m.groups)))
	for {
		matchBitset := metadataMatchH2(&m.ctrlMetadatas[i], h2)
		for /* found */ matchBitset != 0 {
			if /* hash collision */ j := nextMatch(&matchBitset);
			/* key equal, update */ key == m.groups[i].keys[j] {
				m.groups[i].keys[j] = key
				m.groups[i].vals[j] = val
				return
			}
		}

		if /* not found */ matchBitset = metadataMatchEmpty(&m.ctrlMetadatas[i]); /* insert */ matchBitset != 0 {
			n := nextMatch(&matchBitset)
			m.groups[i].keys[n] = key
			m.groups[i].vals[n] = val
			m.ctrlMetadatas[i][n] = int8(h2)
			m.resident++
			return
		}
		i += 1                          // open-addressing (linear-probing) next slot
		if i >= uint32(len(m.groups)) { // wrap-around
			i = 0
		}
	}
}

func (m *SwissMap[K, V]) Get(key K) (val V, exists bool) {
	h1, h2 := splitHash(m.hasher.Hash(key))
	i := linearProbing(h1, uint32(len(m.groups)))
	for {
		matchBitset := metadataMatchH2(&m.ctrlMetadatas[i], h2)
		for matchBitset != 0 {
			j := nextMatch(&matchBitset)
			if key == m.groups[i].keys[j] {
				return m.groups[i].vals[j], true
			}
		}
		if metadataMatchEmpty(&m.ctrlMetadatas[i]) != 0 {
			return val, false
		}
		i += 1
		if i >= uint32(len(m.groups)) { // wrap-around
			i = 0
		}
	}
}

func (m *SwissMap[K, V]) Foreach(action func(i uint64, key K, val V) bool) {
	oldControl, oldGroups := m.ctrlMetadatas, m.groups
	i := randv2.Uint32N(uint32(len(oldGroups)))
	idx := uint64(0)
	for _i := 0; _i < len(oldGroups); _i++ {
		for j, ctrl := range oldControl[i] {
			if ctrl == empty || ctrl == deleted {
				continue
			}
			k, v := oldGroups[i].keys[j], oldGroups[i].vals[j]
			if _continue := action(idx, k, v); !_continue {
				return
			}
			idx++
		}
		i++
		if i >= uint32(len(oldGroups)) { // wrap-around
			i = 0
		}
	}
}

func (m *SwissMap[K, V]) Delete(key K) (val V, err error) {
	h1, h2 := splitHash(m.hasher.Hash(key))
	i := linearProbing(h1, uint32(len(m.groups)))
	for {
		matchBitset := metadataMatchH2(&m.ctrlMetadatas[i], h2)
		for matchBitset != 0 {
			j := nextMatch(&matchBitset)
			if key == m.groups[i].keys[j] {
				if metadataMatchEmpty(&m.ctrlMetadatas[i]) != 0 {
					m.ctrlMetadatas[i][j] = empty
					m.resident--
				} else {
					m.ctrlMetadatas[i][j] = deleted
					m.dead++
				}
				var (
					k K
					v V
				)
				m.groups[i].keys[j] = k
				m.groups[i].vals[j] = v
				return
			}
		}
		// Not found
		if metadataMatchEmpty(&m.ctrlMetadatas[i]) != 0 {
			return val, errors.New("[swiss-map] not found to delete")
		}
		i += 1
		if i >= uint32(len(m.groups)) { // wrap-around
			i = 0
		}
	}
}

func (m *SwissMap[K, V]) Clear() {
	for i, ctrl := range m.ctrlMetadatas {
		for j := range ctrl {
			m.ctrlMetadatas[i][j] = empty
		}
	}
	var (
		k K
		v V
	)
	for i := range m.groups {
		group := &m.groups[i]
		for j := range group.keys {
			group.keys[j] = k
			group.vals[j] = v
		}
	}
	m.resident, m.dead = 0, 0
}

func (m *SwissMap[K, V]) Len() int64 {
	return int64(m.resident - m.dead)
}

func (m *SwissMap[K, V]) Cap() int64 {
	return int64(m.limit - m.resident)
}

func (m *SwissMap[K, V]) nextCap() (uint32, error) {
	if m.dead >= (m.resident >> 1) {
		return uint32(len(m.groups)), nil
	}
	newCap := uint32(len(m.groups)) * 2
	if newCap > math.MaxUint32 {
		return 0, errors.New("[swiss-map] overflow")
	}
	return newCap, nil
}

func (m *SwissMap[K, V]) rehash(newGroups uint32) {
	oldGroups, oldControl := m.groups, m.ctrlMetadatas
	m.groups = make([]swissMapGroup[K, V], newGroups)
	m.ctrlMetadatas = make([]swissMapMetadata, newGroups)
	for i := 0; i < len(m.groups); i++ {
		m.ctrlMetadatas[i] = newEmptyMetadata()
	}

	m.hasher = newSeedHasher[K](m.hasher)
	m.limit = newGroups * maxAvgGroupLoad
	m.resident, m.dead = 0, 0
	for i := range oldControl {
		for j := range oldControl[i] {
			ctrl := oldControl[i][j]
			if ctrl == empty || ctrl == deleted {
				continue
			}
			m.put(oldGroups[i].keys[j], oldGroups[i].vals[j])
		}
	}
}

func (m *SwissMap[K, V]) loadFactor() float64 {
	slots := float64(len(m.groups) * groupSize)
	return float64(m.resident-m.dead) / slots
}

// @param size, how many elements will be stored in the map
func NewSwissMap[K infra.OrderedKey, V any](capacity uint32) *SwissMap[K, V] {
	groups := calcGroups(capacity)
	m := &SwissMap[K, V]{
		ctrlMetadatas: make([]swissMapMetadata, groups),
		groups:       make([]swissMapGroup[K, V], groups),
		hasher:       newHasher[K](),
		resident:     0,
		dead:         0,
		limit:        groups * maxAvgGroupLoad,
	}
	for i := 0; i < len(m.ctrlMetadatas); i++ {
		m.ctrlMetadatas[i] = newEmptyMetadata()
	}
	return m
}

func calcGroups(size uint32) uint32 {
	groups := (size + maxAvgGroupLoad - 1) / maxAvgGroupLoad
	if groups == 0 {
		groups = 1
	}
	return groups
}

func newEmptyMetadata() swissMapMetadata {
	var m swissMapMetadata
	for i := 0; i < len(m); i++ {
		m[i] = empty
	}
	return m
}

func splitHash(hash uint64) (hi h1, lo h2) {
	return h1((hash & h1Mask) >> 7), h2(hash & h2Mask)
}

func linearProbing(hi h1, groups uint32) uint32 {
	return uint32(hi) & (groups - 1)
}
