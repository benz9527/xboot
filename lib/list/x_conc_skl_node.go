package list

import (
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

type vNodeRbtreeColor bool

const (
	red   vNodeRbtreeColor = true
	black vNodeRbtreeColor = false
)

type vNode[V comparable] struct {
	val   *V
	left  *vNode[V] // Linked-list & rbtree
	right *vNode[V] // rbtree only
	color vNodeRbtreeColor
}

const (
	nodeFullyLinkedBit = 1 << iota
	nodeRemovingMarkedBit
	nodeHeadMarkedBit
	vNodeTypeBits = 0x0018
)

type vNodeType uint8

const (
	unique     vNodeType = 0
	linkedList vNodeType = 1
	rbtree     vNodeType = 3
)

type xConcSkipListNode[K infra.OrderedKey, V comparable] struct {
	// If it is unique v-node type store value directly.
	// Otherwise, it is a sentinel node.
	root    *vNode[V]
	key     K
	vcmp    SkipListValueComparator[V]
	indexes xConcSkipListIndices[K, V]
	mu      segmentedMutex
	flags   flagBits
	count   int64
	level   uint32
}

func (node *xConcSkipListNode[K, V]) storeVal(value V) {
	typ := vNodeType(node.flags.atomicLoadBits(vNodeTypeBits))
	switch typ {
	case unique:
		// Replace
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&node.root.val)), unsafe.Pointer(&value))
	case linkedList:
		// predecessor
		pred := node.root
		for n := node.root.left; n != nil; n = n.left {
			res := node.vcmp(value, *n.val)
			if res == 0 {
				// Replace
				pred = n
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.val)), unsafe.Pointer(&value))
				return
			} else if res > 0 {
				pred = n
				if next := n.left; next != nil {
					continue
				}
				// Append
				vn := &vNode[V]{
					val:   &value,
					left:  n.left,
					right: nil,
				}
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.left)), unsafe.Pointer(vn))
				atomic.AddInt64(&node.count, 1)
				break
			} else {
				// Prepend
				vn := &vNode[V]{
					val:   &value,
					left:  n,
					right: nil,
				}
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&pred.left)), unsafe.Pointer(vn))
				atomic.AddInt64(&node.count, 1)
				break
			}
		}
	case rbtree:
		// TODO rbtree store element
	default:
		panic("unknown v-node type")
	}
}

func (node *xConcSkipListNode[K, V]) loadValNode() *vNode[V] {
	return (*vNode[V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&node.root))))
}

func (node *xConcSkipListNode[K, V]) loadNext(i int32) *xConcSkipListNode[K, V] {
	return node.indexes.loadForward(i)
}

func (node *xConcSkipListNode[K, V]) storeNext(i int32, next *xConcSkipListNode[K, V]) {
	node.indexes.storeForward(i, next)
}

func (node *xConcSkipListNode[K, V]) atomicLoadNext(i int32) *xConcSkipListNode[K, V] {
	return node.indexes.atomicLoadForward(i)
}

func (node *xConcSkipListNode[K, V]) atomicStoreNext(i int32, next *xConcSkipListNode[K, V]) {
	node.indexes.atomicStoreForward(i, next)
}

func (node *xConcSkipListNode[K, V]) loadPrev(i int32) *xConcSkipListNode[K, V] {
	return node.indexes.loadBackward(i)
}

func (node *xConcSkipListNode[K, V]) storePrev(i int32, prev *xConcSkipListNode[K, V]) {
	node.indexes.storeBackward(i, prev)
}

func (node *xConcSkipListNode[K, V]) atomicLoadPrev(i int32) *xConcSkipListNode[K, V] {
	return node.indexes.atomicLoadBackward(i)
}

func (node *xConcSkipListNode[K, V]) atomicStorePrev(i int32, prev *xConcSkipListNode[K, V]) {
	node.indexes.atomicStoreBackward(i, prev)
}

func newXConcSkipListNode[K infra.OrderedKey, V comparable](
	key K,
	val V,
	lvl int32,
	mu mutexImpl,
	typ vNodeType,
	cmp SkipListValueComparator[V],
) *xConcSkipListNode[K, V] {
	node := &xConcSkipListNode[K, V]{
		key:   key,
		level: uint32(lvl),
		mu:    mutexFactory(mu),
		vcmp:  cmp,
	}
	node.indexes = newXConcSkipListIndices[K, V](lvl)
	node.flags.setBitsAs(vNodeTypeBits, uint32(typ))
	switch typ {
	case unique:
		node.root = &vNode[V]{
			val:   &val,
			left:  nil,
			right: nil,
		}
	case linkedList:
		node.root = &vNode[V]{
			val: nil,
			left: &vNode[V]{
				val:   &val,
				left:  nil,
				right: nil,
			},
			right: nil,
		}
	case rbtree:
		// TODO rbtree build
	default:
		panic("unknown v-node type")
	}
	node.count = 1
	return node
}

func newXConcSkipListHead[K infra.OrderedKey, V comparable](e mutexImpl, typ vNodeType) *xConcSkipListNode[K, V] {
	head := &xConcSkipListNode[K, V]{
		key:   *new(K),
		level: xSkipListMaxLevel,
		root:  nil,
		mu:    mutexFactory(e),
	}
	head.flags.atomicSet(nodeHeadMarkedBit | nodeFullyLinkedBit)
	head.flags.setBitsAs(vNodeTypeBits, uint32(typ))
	head.indexes = newXConcSkipListIndices[K, V](xSkipListMaxLevel)
	return head
}

func unlockNodes[K infra.OrderedKey, V comparable](version uint64, num int32, nodes ...*xConcSkipListNode[K, V]) {
	var prev *xConcSkipListNode[K, V]
	for i := num; i >= 0; i-- {
		if nodes[i] != prev { // the node could be unlocked by previous loop
			nodes[i].mu.unlock(version)
			prev = nodes[i]
		}
	}
}
