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
	nodeDuplicateBit
	nodeRbtreeBit
)

type vNodeType uint8

const (
	vNodeTypeBits           = 0x0018
	unique        vNodeType = 0
	linkedList    vNodeType = 1
	rbtree        vNodeType = 3
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

func (node *xConcSkipListNode[K, V]) storeVal(val V) {
	typ := vNodeType(node.flags.atomicLoadBits(vNodeTypeBits))
	switch typ {
	case unique:
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&node.root.val)), unsafe.Pointer(&val))
	case linkedList:
		// predecessor
		pred := node.root
		for n := node.root.left; n != nil; {
			res := node.vcmp(val, *n.val)
			if res == 0 {
				// replace
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.val)), unsafe.Pointer(&val))
				pred = n
				return
			} else if res > 0 {
				next := n.left
				if next != nil {
					pred = n
					n = next
					continue
				}
				// append
				vn := &vNode[V]{
					val:   &val,
					left:  nil,
					right: nil,
				}
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.left)), unsafe.Pointer(&vn))
				pred = n
				return
			} else {
				// prepend
				vn := &vNode[V]{
					val:   &val,
					left:  n,
					right: nil,
				}
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&pred.left)), unsafe.Pointer(&vn))
			}
		}
	case rbtree:
		// TODO rbtree store element
	default:
		panic("unknow v-node type")
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

func newXConcSkipListNode[K infra.OrderedKey, V comparable](key K, val V, level int32, e mutexEnum, typ vNodeType) *xConcSkipListNode[K, V] {
	node := &xConcSkipListNode[K, V]{
		key:   key,
		level: uint32(level),
		mu:    mutexFactory(e),
	}
	node.storeVal(val)
	node.indexes = newXConcSkipListIndices[K, V](level)
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
		panic("unknown node type")
	}
	return node
}

func newXConcSkipListHead[K infra.OrderedKey, V comparable](e mutexEnum, typ vNodeType) *xConcSkipListNode[K, V] {
	head := &xConcSkipListNode[K, V]{
		key:   *new(K),
		level: xSkipListMaxLevel,
		root:  nil,
		mu:    mutexFactory(e),
	}
	head.storeVal(*new(V))
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
