package list

import (
	"errors"
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

type vNodeRbtreeColor bool

const (
	red   vNodeRbtreeColor = true
	black vNodeRbtreeColor = false
)

// embedded data-structure
// singly linked-list and rbtree
type vNode[V comparable] struct {
	// parent It is easy for us to backward to access upper level node info.
	parent *vNode[V] // Linked-list & rbtree
	left   *vNode[V] // rbtree only
	right  *vNode[V] // rbtree only
	val    *V        // Unique and comparable
	color  vNodeRbtreeColor
}

func (vn *vNode[V]) linkedListNext() *vNode[V] {
	return vn.parent
}

const (
	nodeFullyLinkedBit = 1 << iota
	nodeRemovingMarkedBit
	nodeHeadMarkedBit
	_duplicate
	_containerType

	fullyLinked   = nodeFullyLinkedBit
	vNodeTypeBits = _duplicate | _containerType
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

func (node *xConcSkipListNode[K, V]) storeVal(ver uint64, val V) (isAppend bool, err error) {
	typ := vNodeType(node.flags.atomicLoadBits(vNodeTypeBits))
	switch typ {
	case unique:
		// Replace
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&node.root.val)), unsafe.Pointer(&val))
	case linkedList:
		// predecessor
		node.mu.lock(ver)
		node.flags.atomicUnset(nodeFullyLinkedBit)
		for pred, n := node.root, node.root.linkedListNext(); n != nil; n = n.linkedListNext() {
			res := node.vcmp(val, *n.val)
			if res == 0 {
				// Replace
				pred = n
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.val)), unsafe.Pointer(&val))
				break
			} else if res > 0 {
				pred = n
				if next := n.parent; next != nil {
					continue
				}
				// Append
				vn := &vNode[V]{
					val:    &val,
					parent: n.parent,
				}
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.parent)), unsafe.Pointer(vn))
				atomic.AddInt64(&node.count, 1)
				isAppend = true
				break
			} else {
				// Prepend
				vn := &vNode[V]{
					val:    &val,
					parent: n,
				}
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&pred.parent)), unsafe.Pointer(vn))
				atomic.AddInt64(&node.count, 1)
				isAppend = true
				break
			}
		}
		node.mu.unlock(ver)
		node.flags.atomicSet(nodeFullyLinkedBit)
	case rbtree:
		// TODO rbtree store element
	default:
		return false, errors.New("unknown v-node type")
	}
	return isAppend, nil
}

func (node *xConcSkipListNode[K, V]) loadVNode() *vNode[V] {
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

/* rbtree operation implementation */

// References:
// https://elixir.bootlin.com/linux/latest/source/lib/rbtree.c

//	 |                         |
//	 N                         S
//	/ \     leftRotate(N)     / \
//
// L   S    ============>    N   R
//
//	 / \                   / \
//	M   R                 L   M
func (node *xConcSkipListNode[K, V]) rbtreeLeftRotate(vnx *vNode[V]) {
	vny := vnx.right
	vnx.right = vny.left

	if vny.left != nil {
		vny.left.parent = vnx
	}

	vny.parent = vnx.parent
	if vnx.parent == nil {
		// vnx is the root node of this rbtree
		// Now, update to set vny as the root node of this rbtree.
		node.root = vny
	} else if vnx == vnx.parent.left {
		// vnx is not the root node of this rbtree,
		// it is a left node of its parent.
		vnx.parent.left = vny
	} else {
		// vnx is not the root node of this rbtree,
		// it is a right node of its parent.
		vnx.parent.right = vny
	}

	vny.left = vnx
	vnx.parent = vny
}

//	 |                         |
//	 N                         S
//	/ \     rightRotate(S)    / \
//
// L   S    <============    N   R
//
//	 / \                   / \
//	M   R                 L   M
func (node *xConcSkipListNode[K, V]) rbtreeRightRotate(vnx *vNode[V]) {
	vny := vnx.left
	vnx.left = vny.right

	if vny.right != nil {
		vny.right.parent = vnx
	}

	vny.parent = vnx.parent
	if vnx.parent == nil {
		// vnx is the root node of this rbtree.
		// Now, update to set vny as the root node of this rbtree.
		node.root = vny
	} else if vnx == vnx.parent.left {
		// vnx is not the root node of this rbtree,
		// it is a left node of its parent.
		vnx.parent.left = vny
	} else {
		// vnx is not the root node of this rbtree,
		// it is a right node of its parent.
		vnx.parent.right = vny
	}

	vny.left = vnx
	vnx.parent = vny
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
			val: &val,
		}
	case linkedList:
		node.root = &vNode[V]{
			parent: &vNode[V]{
				val: &val,
			},
		}
	case rbtree:
		// TODO rbtree build
	default:
		panic("unknown v-node type")
	}
	node.count = 1
	return node
}

func (node *xConcSkipListNode[K, V]) rbtreeInsert(val V) {
	nvn := &vNode[V]{
		val:   &val,
		color: red,
	}

	var (
		vnx, vny *vNode[V] = node.root, nil
	)

	// vnx play as the next level node detector.
	// vny store the current node found by dichotomy.
	for vnx != nil {
		vny = vnx
		// Iterating to find insert position.
		// O(logN)
		if node.vcmp(val, *vnx.val) < 0 {
			vnx = vnx.left
		} else {
			vnx = vnx.right
		}
	}

	// vny is the new vnode's parent
	nvn.parent = vny
	if vny == nil {
		// case 1: Build root node of this rbtree, first inserted element.
		node.root = nvn
	} else if node.vcmp(val, *vny.val) < 0 {
		// Root node of this rbtree exists.
		vny.left = nvn
	} else {
		vny.right = nvn
	}

	if nvn.parent == nil {
		// case 1: Build root node of this rbtree, first inserted element.
		nvn.color = black // root node's color must be black
		return
	} else if nvn.parent.parent == nil {
		// case 2: Root node (black) exists and new node is a child node next to it.
		// New node's parent's parent is nil.
		// It means that new node's parent is the root node
		// of this rbtree (black).
		// New node is red by default.
		// Nil leaf node is black by default.
		return
	}

	node.rbtreePostInsertBalance(nvn)
}

func (node *xConcSkipListNode[K, V]) rbtreePostInsertBalance(nvn *vNode[V]) {
	var aux *vNode[V] = nil
	for nvn.parent.color == red {
		// New node color is red by default.
		// Color adjust from the bottom-up.
		if nvn.parent == nvn.parent.parent.left {
			// New node parent is the left subtree
			aux = nvn.parent.parent.right // uncle node
			if aux.color == red {
				// black (father) <-- <left> -- red (grandfather) -- <right> --> (uncle) black
				aux.color = black             // uncle node
				nvn.parent.color = black      // father node
				nvn.parent.parent.color = red // grandfather node
				nvn = nvn.parent.parent       // backtrack to grandfather node and repaint
			} else {
				// uncle node is black.
				// Height balance
				if nvn == nvn.parent.right {
					// case 1: New node is the opposite direction as parent
					// [] is black, <> is red
					//      [G]                 [G]
					//      / \    rotate(P)    / \
					//    <P> [U]  ========>  <N> [U]
					//      \                 /
					//      <N>             <P>
					nvn = nvn.parent
					node.rbtreeLeftRotate(nvn)
				}
				// case 2: New node is the same direction as parent
				// [] is black, <> is red
				//        [G]                      <P>                 [P]
				//        / \    rightRotate(G)    / \     repaint     / \
				//      <P> [U]  ========>       <N> [G]   ======>   <N> <G>
				//      /                              \                   \
				//    <N>                              [U]                 [U]
				nvn.parent.color = black      // father node
				nvn.parent.parent.color = red // grandfather node
				// Backtrack to adjust the grandfather node's height
				node.rbtreeRightRotate(nvn.parent.parent)
			}
		} else if nvn.parent == nvn.parent.parent.right {
			// New node parent is the right subtree
			// Check left subtree of grandfather node.
			aux = nvn.parent.parent.left // uncle node
			if aux.color == red {
				// black (uncle) <-- <left> -- red (grandfather) -- <right> --> (father) black
				aux.color = black             // uncle node
				nvn.parent.color = black      // father node
				nvn.parent.parent.color = red // grandfather node
				nvn = nvn.parent.parent       // backtrack to grandfather node and repaint
			} else {
				// uncle node is black.
				// Do height balance.
				if nvn == nvn.parent.left {
					// case 1: New node is the opposite direction as parent
					// [] is black, <> is red
					//      [G]                 [G]
					//      / \    rotate(P)    / \
					//    [U] <P>  ========>  [U] <N>
					//        /                     \
					//      <N>                     <P>
					nvn = nvn.parent
					// Here will change the grandfather node
					node.rbtreeRightRotate(nvn)
				}
				// case 2: New node is the same direction as parent
				// [] is black, <> is red
				//        [G]                     <P>                [P]
				//        / \    leftRotate(G)    / \    repaint     / \
				//      [U] <P>  ========>      [G] <N>  ======>   <G> <N>
				//            \                 /                  /
				//            <N>             [U]                [U]
				nvn.parent.color = black      // father node
				nvn.parent.parent.color = red // grandfather node
				// Backtrack to adjust the grandfather node's height
				node.rbtreeLeftRotate(nvn.parent.parent)
			}
		}

		if nvn == node.root {
			break
		}
	}
	node.root.color = black
}

func (node *xConcSkipListNode[K, V]) rbtreeMinimum(vn *vNode[V]) *vNode[V] {
	aux := vn
	for aux.left != nil {
		aux = aux.left
	}
	return aux
}

func (node *xConcSkipListNode[K, V]) rbtreeMaximum(vn *vNode[V]) *vNode[V] {
	aux := vn
	for aux.right != nil {
		aux = aux.right
	}
	return aux
}

// The successor node of the current node is its next node in sorted order.
func (node *xConcSkipListNode[K, V]) rbtreeSucc(vn *vNode[V]) *vNode[V] {
	aux := vn
	if aux.right != nil {
		return node.rbtreeMinimum(aux.right)
	}

	aux = vn.parent
	for aux != nil && vn == aux.right {
		// If vn is father node's left subtree, father node is
		// vn's successor.
		vn = aux
		aux = aux.parent
	}
	return aux
}

// The predecessor node of the current node is its previous node in sorted order
func (node *xConcSkipListNode[K, V]) rbtreePred(vn *vNode[V]) *vNode[V] {
	aux := vn
	if aux.left != nil {
		return node.rbtreeMaximum(aux.left)
	}

	aux = vn.parent
	for aux != nil && vn == aux.left {
		// If vn is father node's right subtree, father node is
		// vn's predecessor.
		vn = aux
		aux = aux.parent
	}
	return aux
}

func (node *xConcSkipListNode[K, V]) rbtreeTransplant(ovn, rvn *vNode[V]) {
	// ovn is ready to be removed.
	// rvn is used to replace ovn.
	if ovn.parent == nil {
		// ovn is root node of this rbtree.
		node.root = rvn
	} else if ovn == ovn.parent.left {
		ovn.parent.left = rvn
	} else if ovn == ovn.parent.right {
		ovn.parent.right = rvn
	}
	rvn.parent = ovn.parent
}

func (node *xConcSkipListNode[K, V]) rbtreeDelete(pvn *vNode[V], val V) error {
	var vnx, vny, vnz *vNode[V] = nil, nil, nil
	// Iteration
	for pvn != nil {
		res := node.vcmp(val, *pvn.val)
		if res == 0 {
			vnz = pvn
		} else if res > 0 {
			pvn = pvn.right
		} else {
			pvn = pvn.left
		}
	}
	if vnz == nil {
		return errors.New("not exists")
	}

	// Found then remove it from rbtree
	vny = vnz // vnz is the remove target node
	yOldColor := vny.color
	if vnz.left == nil && vnz.right != nil {
		// Leaf node
		vnx = vnz.right
		node.rbtreeTransplant(vnz, vnz.right)
	} else if vnz.right == nil && vnz.left != nil {
		// Leaf node
		vnx = vnz.left
		node.rbtreeTransplant(vnz, vnz.left)
	} else if vnz.right != nil && vnz.left != nil {
		// None leaf node
		// Find minimum value node to replace it
		vny = node.rbtreeMinimum(vnz.right) // right minimum with left child
		yOldColor = vny.color

		vnx = vny.right
		if vny.parent == vnz {
			// The minimum value node's father node
			// is the remove target node.
			vnx.parent = vny
		} else {
			node.rbtreeTransplant(vny, vny.right)
			vny.right = vnz.right
			vny.right.parent = vny
		}

		node.rbtreeTransplant(vnz, vny)
		vny.left = vnz.left
		vny.left.parent = vny
		vny.color = vnz.color
	}

	// The remove target node (leaf) is black, we have to rebalance the rbtree.
	if yOldColor == black {
		node.rbtreePostDeleteBalance(vnx)
	}
	// If it is red, directly remove is okay.
	return nil
}

func (node *xConcSkipListNode[K, V]) rbtreePostDeleteBalance(vn *vNode[V]) {
	var aux *vNode[V] = nil
	for vn != node.root && vn.color == black {
		if vn == vn.parent.left {
			aux = vn.parent.right
			if aux.color == red {
				aux.color = black
				vn.parent.color = red
				node.rbtreeLeftRotate(vn.parent)
				aux = vn.parent.right
			}

			if aux.left.color == black && aux.right.color == black {
				aux.color = red
				vn = vn.parent
			} else {
				if aux.right.color == black {
					aux.left.color = black
					aux.color = red
					node.rbtreeRightRotate(aux)
					aux = vn.parent.right
				}

				aux.color = vn.parent.color
				vn.parent.color = black
				aux.right.color = black
				node.rbtreeLeftRotate(vn.parent)
				vn = node.root
			}
		} else {
			aux = vn.parent.left
			if aux.color == red {
				aux.color = black
				vn.parent.color = red
				node.rbtreeRightRotate(vn.parent)
				aux = vn.parent.left
			}

			if aux.left.color == black && aux.right.color == black {
				aux.color = red
				vn = vn.parent
			} else {
				if aux.left.color == black {
					aux.right.color = black
					aux.color = red
					node.rbtreeLeftRotate(aux)
					aux = vn.parent.left
				}

				aux.color = vn.parent.color
				vn.parent.color = black
				aux.left.color = black
				node.rbtreeRightRotate(vn.parent)
				vn = node.root
			}
		}
	}
	vn.color = black
}

func (node *xConcSkipListNode[K, V]) rbtreeSearch(vn *vNode[V], fn func(*vNode[V]) int64) *vNode[V] {
	if vn == nil {
		return nil
	}
	if res := fn(vn); res == 0 {
		return vn
	} else if res > 0 {
		return node.rbtreeSearch(vn.right, fn)
	}
	return node.rbtreeSearch(vn.left, fn)
}

func newXConcSkipListHead[K infra.OrderedKey, V comparable](e mutexImpl, typ vNodeType) *xConcSkipListNode[K, V] {
	head := &xConcSkipListNode[K, V]{
		key:   *new(K),
		level: xSkipListMaxLevel,
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
