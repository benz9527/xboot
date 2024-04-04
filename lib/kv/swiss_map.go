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

// Swiss Table
// Hash slot mapped to the key-val pair slot.
// Short hash from lo bits (1 byte) is an optimization to
// accelerate the hash lookup.
// SSE2 instruction the best performance for linear-probing is
// 16! (https://www.youtube.com/watch?v=ncHmEUmJZf4&t=1449s)

//go:generate go run ./simd/asm.go -out match_metadata.s -stubs match_metadata_amd64.go

const (
	h1Mask    uint64 = 0xffff_ffff_ffff_ff80
	h2Mask    uint64 = 0x0000_0000_0000_007f
	empty     int8   = -128 // 0b1000_0000
	tombstone int8   = -2   // 0b1111_1110
)

type h1 uint64 // A 57 bits hash prefix
type h2 int8   // A 7 bits hash suffix
type bitset uint16

type swissMapMetadata [groupSize]int8

type swissMapGroup[K infra.OrderedKey, V any] struct {
	keys [groupSize]K
	vals [groupSize]V
}

type SwissMap[K infra.OrderedKey, V any] struct {
	control  []swissMapMetadata
	groups   []swissMapGroup[K, V]
	hasher   Hasher[K]
	resident uint32
	dead     uint32
	limit    uint32
}

func (m *SwissMap[K, V]) Put(key K, val V) error {
	if m.resident >= m.limit {
		n, err := m.nextSize()
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
	reminder := linearProbing(h1, uint32(len(m.groups)))
	for {
		matchBitset := metadataMatchH2(&m.control[reminder], h2)
		for matchBitset != 0 {
			n := nextMatch(&matchBitset)
			if /* update */ key == m.groups[reminder].keys[n] {
				m.groups[reminder].keys[n] = key
				m.groups[reminder].vals[n] = val
				return
			}
		}
		// Not found
		matchBitset = metadataMatchEmpty(&m.control[reminder])
		if /* insert */ matchBitset != 0 {
			n := nextMatch(&matchBitset)
			m.groups[reminder].keys[n] = key
			m.groups[reminder].vals[n] = val
			m.control[reminder][n] = int8(h2)
			m.resident++
			return
		}
		reminder += 1                          // open-addressing (linear-probing) next slot
		if reminder >= uint32(len(m.groups)) { // wrap-around
			reminder = 0
		}
	}
}

func (m *SwissMap[K, V]) Get(key K) (val V, exists bool) {
	h1, h2 := splitHash(m.hasher.Hash(key))
	reminder := linearProbing(h1, uint32(len(m.groups)))
	for {
		matchBitset := metadataMatchH2(&m.control[reminder], h2)
		for matchBitset != 0 {
			n := nextMatch(&matchBitset)
			if key == m.groups[reminder].keys[n] {
				return m.groups[reminder].vals[n], true
			}
		}
		if metadataMatchEmpty(&m.control[reminder]) != 0 {
			return val, false
		}
		reminder += 1
		if reminder >= uint32(len(m.groups)) { // wrap-around
			reminder = 0
		}
	}
}

func (m *SwissMap[K, V]) Foreach(action func(i uint64, key K, val V) bool) {
	oldControl, oldGroups := m.control, m.groups
	remainder := randv2.Uint32N(uint32(len(oldGroups)))
	idx := uint64(0)
	for i := 0; i < len(oldGroups); i++ {
		for j, ctrl := range oldControl[remainder] {
			if ctrl == empty || ctrl == tombstone {
				continue
			}
			k, v := oldGroups[remainder].keys[j], oldGroups[remainder].vals[j]
			if !action(idx, k, v) {
				return
			}
			idx++
		}
		remainder++
		if remainder >= uint32(len(oldGroups)) { // wrap-around
			remainder = 0
		}
	}
}

func (m *SwissMap[K, V]) Delete(key K) (val V, err error) {
	h1, h2 := splitHash(m.hasher.Hash(key))
	reminder := linearProbing(h1, uint32(len(m.groups)))
	for {
		matchBitset := metadataMatchH2(&m.control[reminder], h2)
		for matchBitset != 0 {
			n := nextMatch(&matchBitset)
			if key == m.groups[reminder].keys[n] {
				if metadataMatchEmpty(&m.control[reminder]) != 0 {
					m.control[reminder][n] = empty
					m.resident--
				} else {
					m.control[reminder][n] = tombstone
					m.dead++
				}
				var (
					k K
					v V
				)
				m.groups[reminder].keys[n] = k
				m.groups[reminder].vals[n] = v
				return
			}
		}
		// Not found
		if metadataMatchEmpty(&m.control[reminder]) != 0 {
			return val, errors.New("[swiss-map] not found to delete")
		}
		reminder += 1
		if reminder >= uint32(len(m.groups)) { // wrap-around
			reminder = 0
		}
	}
}

func (m *SwissMap[K, V]) Clear() {
	for i, ctrl := range m.control {
		for j := range ctrl {
			m.control[i][j] = empty
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

func (m *SwissMap[K, V]) nextSize() (uint32, error) {
	if m.dead >= (m.resident >> 1) {
		return uint32(len(m.groups)), nil
	}
	nsize := uint32(len(m.groups)) * 2
	if nsize > math.MaxUint32 {
		return 0, errors.New("[swiss-map] overflow")
	}
	return nsize, nil
}

func (m *SwissMap[K, V]) rehash(newGroups uint32) {
	oldGroups, oldControl := m.groups, m.control
	m.groups = make([]swissMapGroup[K, V], newGroups)
	m.control = make([]swissMapMetadata, newGroups)
	for i := 0; i < len(m.groups); i++ {
		m.control[i] = newEmptyMetadata()
	}

	m.hasher = newSeedHasher[K](m.hasher)
	m.limit = newGroups * maxAvgGroupLoad
	m.resident, m.dead = 0, 0
	for md := range oldControl {
		for i := range oldControl[md] {
			ctrl := oldControl[md][i]
			if ctrl == empty || ctrl == tombstone {
				continue
			}
			m.put(oldGroups[md].keys[i], oldGroups[md].vals[i])
		}
	}
}

// @param size, how many elements will be stored in the map
func NewSwissMap[K infra.OrderedKey, V any](size uint32) *SwissMap[K, V] {
	groups := calcGroups(size)
	m := &SwissMap[K, V]{
		control:  make([]swissMapMetadata, groups),
		groups:   make([]swissMapGroup[K, V], groups),
		hasher:   newHasher[K](),
		resident: 0,
		dead:     0,
		limit:    groups * maxAvgGroupLoad,
	}
	for i := 0; i < len(m.control); i++ {
		m.control[i] = newEmptyMetadata()
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
