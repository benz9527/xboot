package list

import (
	"errors"
	"sync/atomic"
	"unsafe"

	"github.com/benz9527/xboot/lib/infra"
)

type color bool

const (
	red   color = true
	black color = false
)

// embedded data-structure
// singly linked-list and rbtree
type xNode[V comparable] struct {
	// parent It is easy for us to backward to access upper level node info.
	parent *xNode[V] // Linked-list & rbtree
	left   *xNode[V] // rbtree only
	right  *xNode[V] // rbtree only
	vptr   *V        // value pointer
	color  color
}

func (n *xNode[V]) linkedListNext() *xNode[V] {
	return n.parent
}

/* rbtree helper methods */

func (n *xNode[V]) isRed() bool {
	return !n.isNilLeaf() && n.color == red
}

func (n *xNode[V]) isBlack() bool {
	return n.isNilLeaf() || n.color == black
}

func (n *xNode[V]) isNilLeaf() bool {
	return n == nil || (n.vptr == nil && n.parent == nil && n.left == nil && n.right == nil)
}

func (n *xNode[V]) isRoot() bool {
	return n != nil && n.parent == nil
}

type rbDirection int8

const (
	left rbDirection = -1 + iota
	root
	right
)

func (n *xNode[V]) direction() rbDirection {
	if n.isRoot() {
		return root
	}
	if n == n.parent.left {
		return left
	}
	return right
}

func (n *xNode[V]) sibling() *xNode[V] {
	dir := n.direction()
	switch dir {
	case left:
		return n.parent.right
	case right:
		return n.parent.left
	default:

	}
	return nil
}

func (n *xNode[V]) hasSibling() bool {
	return !n.isRoot() && n.sibling() != nil
}

func (n *xNode[V]) uncle() *xNode[V] {
	return n.parent.sibling()
}

func (n *xNode[V]) hasUncle() bool {
	return !n.isRoot() && n.parent.hasSibling()
}

func (n *xNode[V]) grandpa() *xNode[V] {
	return n.parent.parent
}

func (n *xNode[V]) hasGrandpa() bool {
	return !n.isRoot() && n.parent.parent != nil
}

func (n *xNode[V]) fixLink() {
	if n.left != nil {
		n.left.parent = n
	}
	if n.right != nil {
		n.right.parent = n
	}
}

func (n *xNode[V]) minimum() *xNode[V] {
	aux := n
	for aux != nil && aux.left != nil {
		aux = aux.left
	}
	return aux
}

func (n *xNode[V]) maximum() *xNode[V] {
	aux := n
	for aux != nil && aux.right != nil {
		aux = aux.right
	}
	return aux
}

// The predecessor node of the current node is its previous node in sorted order
func (n *xNode[V]) pred() *xNode[V] {
	x := n
	if x == nil {
		return nil
	}
	aux := x
	if aux.left != nil {
		return aux.left.maximum()
	}

	aux = x.parent
	// Backtrack to father node that is the x's predecessor.
	for aux != nil && x == aux.left {
		x = aux
		aux = aux.parent
	}
	return aux
}

// The successor node of the current node is its next node in sorted order.
func (n *xNode[V]) succ() *xNode[V] {
	x := n
	if x == nil {
		return nil
	}

	aux := x
	if aux.right != nil {
		return aux.right.minimum()
	}

	aux = x.parent
	// Backtrack to father node that is the x's successor.
	for aux != nil && x == aux.right {
		x = aux
		aux = aux.parent
	}
	return aux
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

type xNodeType uint8

const (
	unique     xNodeType = 0
	linkedList xNodeType = 1
	rbtree     xNodeType = 3
)

type xConcSklNode[K infra.OrderedKey, V comparable] struct {
	// If it is unique v-node type store value directly.
	// Otherwise, it is a sentinel node.
	root    *xNode[V]
	key     K
	vcmp    SklValComparator[V]
	indexes xConcSklIndices[K, V]
	mu      segmentedMutex
	flags   flagBits
	count   int64
	level   uint32
}

func (node *xConcSklNode[K, V]) storeVal(ver uint64, val V) (isAppend bool, err error) {
	typ := xNodeType(node.flags.atomicLoadBits(vNodeTypeBits))
	switch typ {
	case unique:
		// Replace
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&node.root.vptr)), unsafe.Pointer(&val))
	case linkedList:
		// predecessor
		node.mu.lock(ver)
		node.flags.atomicUnset(nodeFullyLinkedBit)
		for pred, n := node.root, node.root.linkedListNext(); n != nil; n = n.linkedListNext() {
			res := node.vcmp(val, *n.vptr)
			if res == 0 {
				// Replace
				pred = n
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.vptr)), unsafe.Pointer(&val))
				break
			} else if res > 0 {
				pred = n
				if next := n.parent; next != nil {
					continue
				}
				// Append
				vn := &xNode[V]{
					vptr:   &val,
					parent: n.parent,
				}
				atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.parent)), unsafe.Pointer(vn))
				atomic.AddInt64(&node.count, 1)
				isAppend = true
				break
			} else {
				// Prepend
				vn := &xNode[V]{
					vptr:   &val,
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

func (node *xConcSklNode[K, V]) loadXNode() *xNode[V] {
	return (*xNode[V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&node.root))))
}

func (node *xConcSklNode[K, V]) loadNext(i int32) *xConcSklNode[K, V] {
	return node.indexes.loadForward(i)
}

func (node *xConcSklNode[K, V]) storeNext(i int32, next *xConcSklNode[K, V]) {
	node.indexes.storeForward(i, next)
}

func (node *xConcSklNode[K, V]) atomicLoadNext(i int32) *xConcSklNode[K, V] {
	return node.indexes.atomicLoadForward(i)
}

func (node *xConcSklNode[K, V]) atomicStoreNext(i int32, next *xConcSklNode[K, V]) {
	node.indexes.atomicStoreForward(i, next)
}

func (node *xConcSklNode[K, V]) loadPrev(i int32) *xConcSklNode[K, V] {
	return node.indexes.loadBackward(i)
}

func (node *xConcSklNode[K, V]) storePrev(i int32, prev *xConcSklNode[K, V]) {
	node.indexes.storeBackward(i, prev)
}

func (node *xConcSklNode[K, V]) atomicLoadPrev(i int32) *xConcSklNode[K, V] {
	return node.indexes.atomicLoadBackward(i)
}

func (node *xConcSklNode[K, V]) atomicStorePrev(i int32, prev *xConcSklNode[K, V]) {
	node.indexes.atomicStoreBackward(i, prev)
}

/* rbtree operation implementation */

// References:
// https://elixir.bootlin.com/linux/latest/source/lib/rbtree.c
// rbtree properties:
// 1. Each node is either red or black.
// 2. The root is black.
// 3. All leaves (and NIL) are black.
// 4. If a red node has children, then the children are black (no two red nodes can be adjacent).
// 5. Every path from a node to its descendant NIL nodes has the same number of black nodes.
// So the shortest path nodes are black nodes. Otherwise,
// the path must contain red node.
// The longest path nodes' number is 2 * shortest path nodes' number.

//	 |                         |
//	 X                         S
//	/ \     leftRotate(X)     / \
//
// L   S    ============>    X   R
//
//	 / \                   / \
//	M   R                 L   M
func (node *xConcSklNode[K, V]) rbLeftRotate(x *xNode[V]) {
	if x == nil || x.right == nil {
		panic("rbtree left rotate node x is nil or x.right is nil")
	}

	p, y := x.parent, x.right
	dir := x.direction()
	x.right, y.left = y.left, x

	x.fixLink()
	y.fixLink()

	switch dir {
	case root:
		node.root = y
	case left:
		p.left = y
	case right:
		p.right = y
	}
	y.parent = p
}

//	 |                         |
//	 X                         S
//	/ \     rightRotate(S)    / \
//
// L   S    <============    X   R
//
//	 / \                   / \
//	M   R                 L   M
func (node *xConcSklNode[K, V]) rbRightRotate(x *xNode[V]) {
	if x == nil || x.left == nil {
		panic("rbtree right rotate node x is nil or x.right is nil")
	}

	p, y := x.parent, x.left
	dir := x.direction()
	x.left, y.right = y.right, x

	x.fixLink()
	y.fixLink()

	switch dir {
	case root:
		node.root = y
	case left:
		p.left = y
	case right:
		p.right = y
	}
	y.parent = p
}

func (node *xConcSklNode[K, V]) rbInsert(val V) {

	if node.root.isNilLeaf() {
		node.root = &xNode[V]{
			vptr: &val,
		}
		atomic.AddInt64(&node.count, 1)
		return
	}

	var x, y *xNode[V] = node.root, nil
	for !x.isNilLeaf() {
		y = x
		res := node.vcmp(val, *x.vptr)
		if /* Equal */ res == 0 {
			break
		} else /* Less */ if res < 0 {
			x = x.left
		} else /* Greater */ {
			x = x.right
		}
	}

	// assert y != nil
	var z *xNode[V]
	res := node.vcmp(val, *y.vptr)
	if /* Equal */ res == 0 {
		y.vptr = &val
		return
	} else /* Less */ if res < 0 {
		z = &xNode[V]{
			vptr:   &val,
			color:  red,
			parent: y,
		}
		y.left = z
	} else /* Greater */ {
		z = &xNode[V]{
			vptr:   &val,
			color:  red,
			parent: y,
		}
		y.right = z
	}

	atomic.AddInt64(&node.count, 1)
	node.rbPostInsertBalance(z)
}

// New node color is red by default.
// Color adjust from the bottom-up.
func (node *xConcSklNode[K, V]) rbPostInsertBalance(x *xNode[V]) {
	if x.isRoot() {
		if x.isRed() {
			x.color = black
		}
		return
	}

	if x.parent.isBlack() {
		return
	}

	if x.parent.isRoot() {
		if x.parent.isRed() {
			x.parent.color = black
		}
		return
	}

	//        [G]             <G>
	//        / \             / \
	//      <P> <U>  ====>  [P] [U]
	//      /               /
	//    <X>             <X>
	if x.hasUncle() && x.uncle().isRed() {
		x.parent.color = black
		x.uncle().color = black
		gp := x.grandpa()
		gp.color = red
		node.rbPostInsertBalance(gp) // tail recursive
	} else {
		if !x.hasUncle() || x.uncle().isBlack() {
			//      [G]                 [G]
			//      / \    rotate(P)    / \
			//    <P> [U]  ========>  <X> [U]
			//      \                 /
			//      <X>             <P>
			dir := x.direction()
			if dir != x.parent.direction() {
				p := x.parent
				if dir == left {
					node.rbRightRotate(p)
				} else /* Right */ {
					node.rbLeftRotate(p)
				}
				x = p
			}
			//        [G]                 <P>               [P]
			//        / \    rotate(G)    / \    repaint    / \
			//      <P> [U]  ========>  <X> [G]  ======>  <X> <G>
			//      /                         \                 \
			//    <X>                         [U]               [U]
			dir = x.parent.direction()
			if dir == left {
				node.rbRightRotate(x.grandpa())
			} else /* Right */ {
				node.rbLeftRotate(x.grandpa())
			}

			x.parent.color = black
			x.sibling().color = red
			return
		}
	}
}

func (node *xConcSklNode[K, V]) rbRemove(val V) (res *xNode[V], err error) {
	if atomic.LoadInt64(&node.count) <= 0 {
		return nil, errors.New("empty rbtree")
	}
	z := node.rbSearch(node.root, func(vn *xNode[V]) int64 {
		return node.vcmp(val, *vn.vptr)
	})
	if z == nil {
		return nil, errors.New("not exists")
	}
	defer func() {
		atomic.AddInt64(&node.count, -1)
	}()
	var y *xNode[V] = nil

	// Found z is the remove target node
	// case 1: z is the root node of rbtree, remove directly
	if atomic.LoadInt64(&node.count) == 1 && z.isRoot() {
		node.root = nil
		z.left = nil
		z.right = nil
		return z, nil
	}

	res = &xNode[V]{
		vptr: z.vptr,
	}

	y = z
	// case 2: y contains 2 not nil leaf node
	if !y.left.isNilLeaf() && !y.right.isNilLeaf() {
		// Find the predecessor then swap value only
		//     |                    |
		//     N                    L
		//    / \                  / \
		//   L  ..   swap(N, L)   N  ..
		//       |   =========>       |
		//       P                    P
		//      / \                  / \
		//     S  ..                S  ..
		y = z.pred()
		// Swap value only.
		z.vptr = y.vptr
	}

	// case 3: y is a leaf node.
	if y.left.isNilLeaf() && y.right.isNilLeaf() {
		if y.isBlack() {
			node.rbRemoveBalance(y)
		} else if y.isRed() {
			// Leaf red node, remove directly.
			if y == y.parent.left {
				y.parent.left = nil
			} else if y == y.parent.right {
				y.parent.right = nil
			}
			return res, nil
		}
	} else {
		// case 4: y is not a leaf node.
		replace := &xNode[V]{}
		if !y.right.isNilLeaf() {
			replace = y.right // Maybe a red node
		} else if !y.left.isNilLeaf() {
			replace = y.left // Maybe a red node
		}

		if replace == nil {
			panic("rbtree remove with nil replace node")
		}

		if y.isRoot() {
			// Root node of rbtree
			node.root = replace
		} else if y == y.parent.left {
			y.parent.left = replace
			replace.parent = y.parent
		} else /* y == y.parent.right */ {
			y.parent.right = replace
			replace.parent = y.parent
		}

		if y.color == black {
			if replace.color == red {
				replace.color = black
			} else {
				node.rbRemoveBalance(replace)
			}
		}
	}

	// Unlink node
	if y == y.parent.left {
		y.parent.left = nil
	} else if y == y.parent.right {
		y.parent.right = nil
	}

	y.parent = nil
	y.left = nil
	y.right = nil

	return res, nil
}

func (node *xConcSklNode[K, V]) rbRemoveBalance(x *xNode[V]) {
	if x.isRoot() {
		// Backtrack to root node
		return
	}

	sibling := x.sibling()
	dir := x.direction()
	if sibling.isRed() {
		// case 1: x's sibling node is red
		if dir == left {
			// [] is black, <> is red
			//     |                     |
			//    [P]                   [S]
			//    / \                  /   \
			// [N]  <S>               <P>  [Sr]
			//      /  \   =======>  /  \
			//    [Sl] [Sr]        [N]  [Sl]
			// rotate father node
			node.rbLeftRotate(x.parent)
		} else if dir == right {
			node.rbRightRotate(x.parent)
		}
		sibling.color = black
		x.parent.color = red
		sibling = x.sibling()
	}

	var sc, sd *xNode[V] = nil, nil
	if dir == left {
		sc = sibling.left
		sd = sibling.right
	} else if dir == right {
		sc = sibling.right
		sd = sibling.left
	}

	// sibling must be black

	if sc.isBlack() && sd.isBlack() {
		if x.parent.isRed() {
			//      <P>             [P]
			//      / \             / \
			//    [N] [S]  ====>  [N] <S>
			//        / \             / \
			//     [Sl] [Sr]       [Sl] [Sr]
			sibling.color = red
			x.parent.color = black
			return
		}
		//      [P]             [P]
		//      / \             / \
		//    [N] [S]  ====>  [N] <S>
		//        / \             / \
		//      [C] [D]         [C] [D]
		sibling.color = red
		node.rbRemoveBalance(x.parent)
		return
	} else {
		if !sc.isNilLeaf() && sc.isRed() {
			if dir == left {
				//                            {P}                {P}
				//      {P}                   / \                / \
				//      / \    r-rotate(S)  [N] <Sl>   repaint  [N] [Sl]
				//    [N] [S]  ==========>        \    ======>       \
				//        / \                     [S]                <S>
				//     <Sl> [Sr]                    \                  \
				//                                  [Sr]                [Sr]
				node.rbRightRotate(sibling)
			} else if dir == right {
				node.rbLeftRotate(sibling)
			}
			sc.color = black
			sibling.color = red
			sibling = x.sibling()

			if dir == left {
				sc = sibling.left
				sd = sibling.right
			} else if dir == right {
				sc = sibling.right
				sd = sibling.left
			}
		}

		if dir == left {
			//      {P}                   [S]
			//      / \    l-rotate(P)    / \
			//    [N] [S]  ==========>  {P} <D>
			//        / \               / \
			//      [C] <D>           [N] [C]
			node.rbLeftRotate(x.parent)
		} else if dir == right {
			node.rbRightRotate(x.parent)
		}
		sibling.color = x.parent.color
		x.parent.color = black
		if !sd.isNilLeaf() {
			sd.color = black
		}
		return
	}
}

func (node *xConcSklNode[K, V]) rbSearch(x *xNode[V], fn func(*xNode[V]) int64) *xNode[V] {
	if x == nil {
		return nil
	}

	aux := x
	for aux != nil {
		res := fn(aux)
		if res == 0 {
			return aux
		} else if res > 0 {
			aux = aux.right
		} else {
			aux = aux.left
		}
	}
	return nil
}

func (node *xConcSklNode[K, V]) rbPreorderTraversal(fn func(idx int64, color color, val V) bool) {
	size := atomic.LoadInt64(&node.count)
	aux := node.root
	if size < 0 || aux == nil {
		return
	}
	stack := make([]*xNode[V], 0, size>>1)
	defer func() {
		clear(stack)
	}()
	for aux != nil {
		stack = append(stack, aux)
		aux = aux.left
	}
	idx := int64(0)
	size = int64(len(stack))
	for size > 0 {
		aux = stack[size-1]
		if !fn(idx, aux.color, *aux.vptr) {
			return
		}
		idx++
		stack = stack[:size-1]
		if aux.right != nil {
			aux = aux.right
			for aux != nil {
				stack = append(stack, aux)
				aux = aux.left
			}
		}
		size = int64(len(stack))
	}
}

func (node *xConcSklNode[K, V]) rbInorderTraversal(fn func(idx int64, color color, val V) bool) {

}

func (node *xConcSklNode[K, V]) rbPostorderTraversal(fn func(idx int64, color color, val V) bool) {

}

func newXConcSkipListNode[K infra.OrderedKey, V comparable](
	key K,
	val V,
	lvl int32,
	mu mutexImpl,
	typ xNodeType,
	cmp SklValComparator[V],
) *xConcSklNode[K, V] {
	node := &xConcSklNode[K, V]{
		key:   key,
		level: uint32(lvl),
		mu:    mutexFactory(mu),
		vcmp:  cmp,
	}
	node.indexes = newXConcSkipListIndices[K, V](lvl)
	node.flags.setBitsAs(vNodeTypeBits, uint32(typ))
	switch typ {
	case unique:
		node.root = &xNode[V]{
			vptr: &val,
		}
	case linkedList:
		node.root = &xNode[V]{
			parent: &xNode[V]{
				vptr: &val,
			},
		}
	case rbtree:
		node.rbInsert(val)
	default:
		panic("unknown v-node type")
	}
	node.count = 1
	return node
}

func newXConcSklHead[K infra.OrderedKey, V comparable](e mutexImpl, typ xNodeType) *xConcSklNode[K, V] {
	head := &xConcSklNode[K, V]{
		key:   *new(K),
		level: xSkipListMaxLevel,
		mu:    mutexFactory(e),
	}
	head.flags.atomicSet(nodeHeadMarkedBit | nodeFullyLinkedBit)
	head.flags.setBitsAs(vNodeTypeBits, uint32(typ))
	head.indexes = newXConcSkipListIndices[K, V](xSkipListMaxLevel)
	return head
}

func unlockNodes[K infra.OrderedKey, V comparable](version uint64, num int32, nodes ...*xConcSklNode[K, V]) {
	var prev *xConcSklNode[K, V]
	for i := num; i >= 0; i-- {
		if nodes[i] != prev { // the node could be unlocked by previous loop
			nodes[i].mu.unlock(version)
			prev = nodes[i]
		}
	}
}
