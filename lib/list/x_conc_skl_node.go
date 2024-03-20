package list

import (
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

func (n *xNode[V]) isLeaf() bool {
	return n != nil && n.parent != nil && n.left.isNilLeaf() && n.right.isNilLeaf()
}

func (n *xNode[V]) isRoot() bool {
	return n != nil && n.parent == nil
}

func (n *xNode[V]) loadLeft() *xNode[V] {
	if n.isNilLeaf() {
		return nil
	}
	return n.left
}

func (n *xNode[V]) loadRight() *xNode[V] {
	if n.isNilLeaf() {
		return nil
	}
	return n.right
}

type rbDirection int8

const (
	left rbDirection = -1 + iota
	root
	right
)

func (n *xNode[V]) direction() rbDirection {
	if n.isNilLeaf() {
		panic("[x-conc-skl] rbtree nil leaf node without direction")
	}

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
	for ; aux != nil && aux.left != nil; aux = aux.left {
	}
	return aux
}

func (n *xNode[V]) maximum() *xNode[V] {
	aux := n
	for ; aux != nil && aux.right != nil; aux = aux.right {
	}
	return aux
}

// The pred node of the current node is its previous node in sorted order
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
	// Backtrack to father node that is the x's pred.
	for aux != nil && x == aux.left {
		x = aux
		aux = aux.parent
	}
	return aux
}

// The succ node of the current node is its next node in sorted order.
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
	// Backtrack to father node that is the x's succ.
	for aux != nil && x == aux.right {
		x = aux
		aux = aux.parent
	}
	return aux
}

const (
	nodeInsertedFlagBit = 1 << iota
	nodeRemovingFlagBit
	nodeIsHeadFlagBit
	nodeIsSetFlagBit      /* 0: unique; 1: enable linked-list or rbtree */
	nodeSetModeFlagBit    /* 0: linked-list; 1: rbtree */
	nodeRbRmBorrowFlagBit /* 0: pred; 1: succ */

	insertFullyLinked = nodeInsertedFlagBit
	xNodeModeFlagBits = nodeIsSetFlagBit | nodeSetModeFlagBit
)

type xNodeMode uint8

const (
	unique     xNodeMode = 0
	linkedList xNodeMode = 1
	rbtree     xNodeMode = 3
)

type xConcSklNode[K infra.OrderedKey, V comparable] struct {
	// If it is unique x-node type store value directly.
	// Otherwise, it is a sentinel node for linked-list or rbtree.
	root    *xNode[V]
	key     K
	vcmp    SklValComparator[V]
	indices xConcSklIndices[K, V]
	mu      segmentedMutex
	flags   flagBits
	count   int64 // The number of duplicate elements
	level   uint32
}

func (node *xConcSklNode[K, V]) storeVal(ver uint64, val V, ifNotPresent ...bool) (isAppend bool, err error) {
	switch mode := xNodeMode(node.flags.atomicLoadBits(xNodeModeFlagBits)); mode {
	case unique:
		if ifNotPresent[0] {
			return false, ErrXSklDisabledValReplace
		}
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&node.root.vptr)), unsafe.Pointer(&val))
	case linkedList:
		// pred
		node.mu.lock(ver)
		node.flags.atomicUnset(nodeInsertedFlagBit)
		isAppend, err = node.llInsert(val, ifNotPresent...)
		node.mu.unlock(ver)
		node.flags.atomicSet(nodeInsertedFlagBit)
	case rbtree:
		node.mu.lock(ver)
		node.flags.atomicUnset(nodeInsertedFlagBit)
		isAppend, err = node.rbInsert(val)
		node.mu.unlock(ver)
		node.flags.atomicSet(nodeInsertedFlagBit)
	default:
		// impossible run to here
		panic("[x-conc-skl] unknown x-node type")
	}
	return isAppend, err
}

func (node *xConcSklNode[K, V]) atomicLoadRoot() *xNode[V] {
	return (*xNode[V])(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&node.root))))
}

func (node *xConcSklNode[K, V]) loadNextNode(i int32) *xConcSklNode[K, V] {
	return node.indices.loadForwardIndex(i)
}

func (node *xConcSklNode[K, V]) storeNextNode(i int32, next *xConcSklNode[K, V]) {
	node.indices.storeForwardIndex(i, next)
}

func (node *xConcSklNode[K, V]) atomicLoadNextNode(i int32) *xConcSklNode[K, V] {
	return node.indices.atomicLoadForwardIndex(i)
}

func (node *xConcSklNode[K, V]) atomicStoreNextNode(i int32, next *xConcSklNode[K, V]) {
	node.indices.atomicStoreForwardIndex(i, next)
}

/* linked-list operation implementation */

func (node *xConcSklNode[K, V]) llInsert(val V, ifNotPresent ...bool) (isAppend bool, err error) {
	for pred, n := node.root, node.root.linkedListNext(); n != nil; n = n.linkedListNext() {
		if /* replace */ res := node.vcmp(val, *n.vptr); res == 0 {
			if /* disabled */ ifNotPresent[0] {
				return false, ErrXSklDisabledValReplace
			}
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.vptr)), unsafe.Pointer(&val))
			break
		} else /* append */ if res > 0 {
			pred = n
			if next := n.parent; next != nil {
				continue
			}
			x := &xNode[V]{
				vptr:   &val,
				parent: n.parent,
			}
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&n.parent)), unsafe.Pointer(x))
			atomic.AddInt64(&node.count, 1)
			isAppend = true
			break
		} else /* prepend */ {
			x := &xNode[V]{
				vptr:   &val,
				parent: n,
			}
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&pred.parent)), unsafe.Pointer(x))
			atomic.AddInt64(&node.count, 1)
			isAppend = true
			break
		}
	}
	return isAppend, nil
}

/* rbtree operation implementation */

// References:
// https://elixir.bootlin.com/linux/latest/source/lib/rbtree.c
// rbtree properties:
// https://en.wikipedia.org/wiki/Red%E2%80%93black_tree#Properties
// p1. Every node is either red or black.
// p2. All NIL nodes are considered black.
// p3. A red node does not have a red child. (red-violation)
// p4. Every path from a given node to any of its descendant
//   NIL nodes goes through the same number of black nodes. (black-violation)
// p5. (Optional) The root is black.
// (Conclusion) If a node X has exactly one child, it must be a red child,
//   because if it were black, its NIL descendants would sit at a different
//   black depth than X's NIL child, violating p4.
// So the shortest path nodes are black nodes. Otherwise,
// the path must contain red node.
// The longest path nodes' number is 2 * shortest path nodes' number.

/*
		 |                         |
		 X                         S
		/ \     leftRotate(X)     / \
	   L   S    ============>    X   Sd
		  / \                   / \
		Sc   Sd                L   Sc
*/
func (node *xConcSklNode[K, V]) rbLeftRotate(x *xNode[V]) {
	if x == nil || x.right.isNilLeaf() {
		panic("[x-conc-skl] rbtree left rotate node x is nil or x.right is nil")
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

/*
			 |                         |
			 X                         S
			/ \     rightRotate(S)    / \
	       L   S    <============    X   R
			  / \                   / \
			Sc   Sd               Sc   Sd
*/
func (node *xConcSklNode[K, V]) rbRightRotate(x *xNode[V]) {
	if x == nil || x.left.isNilLeaf() {
		panic("[x-conc-skl] rbtree right rotate node x is nil or x.right is nil")
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

// i1: Empty rbtree, insert directly, but root node is painted to black.
func (node *xConcSklNode[K, V]) rbInsert(val V, ifNotPresent ...bool) (isAppend bool, err error) {

	if /* i1 */ node.root.isNilLeaf() {
		node.root = &xNode[V]{
			vptr: &val,
		}
		atomic.AddInt64(&node.count, 1)
		return true, nil
	}

	var x, y *xNode[V] = node.root, nil
	for !x.isNilLeaf() {
		y = x
		res := node.vcmp(val, *x.vptr)
		if /* equal */ res == 0 {
			break
		} else /* less */ if res < 0 {
			x = x.left
		} else /* greater */ {
			x = x.right
		}
	}

	if y.isNilLeaf() {
		panic("[x-conc-skl] rbtree insert a new value into nil node")
	}

	var z *xNode[V]
	res := node.vcmp(val, *y.vptr)
	if /* equal */ res == 0 {
		if /* disabled */ ifNotPresent[0] {
			return false, ErrXSklDisabledValReplace
		}
		y.vptr = &val
		return false, nil
	} else /* less */ if res < 0 {
		z = &xNode[V]{
			vptr:   &val,
			color:  red,
			parent: y,
		}
		y.left = z
	} else /* greater */ {
		z = &xNode[V]{
			vptr:   &val,
			color:  red,
			parent: y,
		}
		y.right = z
	}

	atomic.AddInt64(&node.count, 1)
	node.rbInsertRebalance(z)
	return true, nil
}

/*
New node X is red by default.

<X> is a RED node.
[X] is a BLACK node (or NIL).
{X} is either a RED node or a BLACK node.

im1: Current node X's parent P is black and P is root, so hold r3 and r4.

im2: Current node X's parent P is red and P is root, repaint P into black.

im3: If both the parent P and the uncle U are red, grandpa G is black.
(red-violation)
After repainted G into red may be still red-violation.
Recursive to fix grandpa.

	    [G]             <G>
	    / \             / \
	  <P> <U>  ====>  [P] [U]
	  /               /
	<X>             <X>

im4: The parent P is red but the uncle U is black. (red-violation)
X is opposite direction to P. Rotate P to opposite direction.
After rotation may be still red-violation. Here must enter im5 to fix.

	  [G]                 [G]
	  / \    rotate(P)    / \
	<P> [U]  ========>  <X> [U]
	  \                 /
	  <X>             <P>

im5: Handle im4 scenario, current node is the same direction as parent.

	    [G]                 <P>               [P]
	    / \    rotate(G)    / \    repaint    / \
	  <P> [U]  ========>  <X> [G]  ======>  <X> <G>
	  /                         \                 \
	<X>                         [U]               [U]
*/
func (node *xConcSklNode[K, V]) rbInsertRebalance(x *xNode[V]) {
	for !x.isNilLeaf() {
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
			if /* im1 */ x.parent.isBlack() {
				return
			} else /* im2 */ {
				x.parent.color = black
			}
		}

		if /* im3 */ x.hasUncle() && x.uncle().isRed() {
			x.parent.color = black
			x.uncle().color = black
			gp := x.grandpa()
			gp.color = red
			x = gp
			continue
		} else {
			if !x.hasUncle() || x.uncle().isBlack() {
				dir := x.direction()
				if /* im4 */ dir != x.parent.direction() {
					p := x.parent
					switch dir {
					case left:
						node.rbRightRotate(p)
					case right:
						node.rbLeftRotate(p)
					default:
						panic("[x-conc-skl] rbtree insert violate (im4)")
					}
					x = p // enter im5 to fix
				}

				switch /* im5 */ dir = x.parent.direction(); dir {
				case left:
					node.rbRightRotate(x.grandpa())
				case right:
					node.rbLeftRotate(x.grandpa())
				default:
					panic("[x-conc-skl] rbtree insert violate (im5)")
				}

				x.parent.color = black
				x.sibling().color = red
				return
			}
		}
	}
}

/*
r1: Only a root node, remove directly.

r2: Current node X has left and right node.
Find node X's pred or succ to replace it to be removed.
Swap the value only.
Both of pred and succ are nil left and right node.

Find pred:

	  |                    |
	  X                    L
	 / \                  / \
	L  ..   swap(X, L)   X  ..
		|   =========>       |
		P                    P
	   / \                  / \
	  S  ..                S  ..

Find succ:

	  |                    |
	  X                    S
	 / \                  / \
	L  ..   swap(X, S)   L  ..
		|   =========>       |
		P                    P
	   / \                  / \
	  S  ..                X  ..

r3: (1) Current node X is a red leaf node, remove directly.

r3: (2) Current node X is a black leaf node, we have to rebalance after remove.
(black-violation)

r4: Current node X is not a leaf node but contains a not nil child node.
The child node must be a red node. (See conclusion. Otherwise, black-violation)
*/
func (node *xConcSklNode[K, V]) rbRemoveNode(z *xNode[V]) (res *xNode[V], err error) {
	if /* r1 */ atomic.LoadInt64(&node.count) == 1 && z.isRoot() {
		node.root = nil
		z.left = nil
		z.right = nil
		return z, nil
	}

	res = &xNode[V]{
		vptr: z.vptr,
	}

	y := z
	if /* r2 */ !y.left.isNilLeaf() && !y.right.isNilLeaf() {
		if node.flags.isSet(nodeRbRmBorrowFlagBit) {
			y = z.succ() // enter r3-r4
		} else {
			y = z.pred() // enter r3-r4
		}
		// Swap value only.
		z.vptr = y.vptr
	}

	if /* r3 */ y.isLeaf() {
		if /* r3 (1) */ y.isRed() {
			switch dir := y.direction(); dir {
			case left:
				y.parent.left = nil
			case right:
				y.parent.right = nil
			default:
				panic("[x-conc-skl] rbtree x-node y should be a leaf node, violate (r3-1)")
			}
			return res, nil
		} else /* r3 (2) */ {
			node.rbRemoveRebalance(y)
		}
	} else /* r4 */ {
		var replace *xNode[V]
		if !y.right.isNilLeaf() {
			replace = y.right
		} else if !y.left.isNilLeaf() {
			replace = y.left
		}

		if replace == nil {
			panic("[x-conc-skl] rbtree remove a leaf node without child, violate (r4)")
		}

		switch dir := y.direction(); dir {
		case root:
			node.root = replace
			node.root.parent = nil
		case left:
			y.parent.left = replace
			replace.parent = y.parent
		case right:
			y.parent.right = replace
			replace.parent = y.parent
		default:
			panic("[x-conc-skl] rbtree impossible run to here")
		}

		if y.isBlack() {
			if replace.isRed() {
				replace.color = black
			} else {
				node.rbRemoveRebalance(replace)
			}
		}
	}

	// Unlink node
	if !y.isRoot() && y == y.parent.left {
		y.parent.left = nil
	} else if !y.isRoot() && y == y.parent.right {
		y.parent.right = nil
	}
	y.parent = nil
	y.left = nil
	y.right = nil

	return res, nil
}

func (node *xConcSklNode[K, V]) rbRemove(val V) (*xNode[V], error) {
	if atomic.LoadInt64(&node.count) <= 0 {
		return nil, ErrXSklNotFound
	}
	z := node.rbSearch(node.root, func(vn *xNode[V]) int64 {
		return node.vcmp(val, *vn.vptr)
	})
	if z == nil {
		return nil, ErrXSklNotFound
	}
	defer func() {
		atomic.AddInt64(&node.count, -1)
	}()

	return node.rbRemoveNode(z)
}

func (node *xConcSklNode[K, V]) rbRemoveMin() (*xNode[V], error) {
	if atomic.LoadInt64(&node.count) <= 0 {
		return nil, ErrXSklNotFound
	}
	_min := node.root.minimum()
	if _min.isNilLeaf() {
		return nil, ErrXSklNotFound
	}
	defer func() {
		atomic.AddInt64(&node.count, -1)
	}()
	return node.rbRemoveNode(_min)
}

/*
<X> is a RED node.
[X] is a BLACK node (or NIL).
{X} is either a RED node or a BLACK node.

Sc is the same direction to X and it X's sibling's child node.
Sd is the opposite direction to X and it X's sibling's child node.

rm1: Current node X's sibling S is red, so the parent P, nephew node Sc and Sd
must be black. (Otherwise, red-violation)
(1) X is left node of P, left rotate P
(2) X is right node of P, right rotate P.
(3) repaint S into black, P into red.

	  [P]                   <S>               [S]
	  / \    l-rotate(P)    / \    repaint    / \
	[X] <S>  ==========>  [P] [D]  ======>  <P> [Sd]
	    / \               / \               / \
	 [Sc] [Sd]          [X] [Sc]          [X] [Sc]

rm2: Current node X's parent P is red, the sibling S, nephew node Sc and Sd
is black.
Repaint S into red and P into black.

	  <P>             [P]
	  / \             / \
	[X] [S]  ====>  [X] <S>
	    / \             / \
	 [Sc] [Sd]       [Sc] [Sd]

rm3: All of current node X's parent P, the sibling S, nephew node Sc and Sd
are black.
Unable to satisfy p3 and p4. We have to paint the S into red to satisfy
p4 locally. Then recursive to handle P.

	  [P]             [P]
	  / \             / \
	[X] [S]  ====>  [X] <S>
	    / \             / \
	 [Sc] [Sd]       [Sc] [Sd]

rm4: Current node X's sibling S is black, nephew node Sc is red and Sd
is black. Ignore X's parent P's color (red or black is okay)
Unable to satisfy p3 and p4.
(1) If X is left node of P, right rotate P.
(2) If X is right node of P, left rotate P.
(3) Repaint S into red, Sc into black
Enter into rm5 to fix.

	                        {P}                {P}
	  {P}                   / \                / \
	  / \    r-rotate(S)  [X] <Sc>   repaint  [X] [Sc]
	[X] [S]  ==========>        \    ======>       \
	    / \                     [S]                <S>
	  <Sc> [Sd]                   \                  \
	                              [Sd]               [Sd]

rm5: Current node X's sibling S is black, nephew node Sc is black and Sd
is red. Ignore X's parent P's color (red or black is okay)
Unable to satisfy p4 (black-violation)
(1) If X is left node of P, left rotate P.
(2) If X is right node of P, right rotate P.
(3) Swap P and S's color (red-violation)
(4) Repaint Sd into black.

	  {P}                   [S]                {S}
	  / \    l-rotate(P)    / \     repaint    / \
	[X] [S]  ==========>  {P} <Sd>  ======>  [P] [Sd]
	    / \               / \                / \
	 [Sc] <Sd>          [X] [Sc]           [X] [Sc]
*/
func (node *xConcSklNode[K, V]) rbRemoveRebalance(x *xNode[V]) {
	for {
		if x.isRoot() {
			return
		}

		sibling := x.sibling()
		dir := x.direction()
		if /* rm1 */ sibling.isRed() {
			switch dir {
			case left:
				node.rbLeftRotate(x.parent)
			case right:
				node.rbRightRotate(x.parent)
			default:
				panic("[x-conc-skl] rbtree remove violate (rm1)")
			}
			sibling.color = black
			x.parent.color = red // ready to enter rm2
			sibling = x.sibling()
		}

		var sc, sd *xNode[V]
		switch /* rm2 */ dir {
		case left:
			sc, sd = sibling.loadLeft(), sibling.loadRight()
		case right:
			sc, sd = sibling.loadRight(), sibling.loadLeft()
		default:
			panic("[x-conc-skl] rbtree remove violate (rm2)")
		}

		if sc.isBlack() && sd.isBlack() {
			if /* rm2 */ x.parent.isRed() {
				sibling.color = red
				x.parent.color = black
				break
			} else /* rm3 */ {
				sibling.color = red
				x = x.parent
				continue
			}
		} else {
			if /* rm 4 */ !sc.isNilLeaf() && sc.isRed() {
				switch dir {
				case left:
					node.rbRightRotate(sibling)
				case right:
					node.rbLeftRotate(sibling)
				default:
					panic("[x-conc-skl] rbtree remove violate (rm4)")
				}
				sc.color = black
				sibling.color = red
				sibling = x.sibling()
				switch dir {
				case left:
					sd = sibling.loadRight()
				case right:
					sd = sibling.loadLeft()
				default:
					panic("[x-conc-skl] rbtree remove violate (rm4)")
				}
			}

			switch /* rm5 */ dir {
			case left:
				node.rbLeftRotate(x.parent)
			case right:
				node.rbRightRotate(x.parent)
			default:
				panic("[x-conc-skl] rbtree remove violate (rm5)")
			}
			sibling.color = x.parent.color
			x.parent.color = black
			if !sd.isNilLeaf() {
				sd.color = black
			}
			break
		}
	}
}

func (node *xConcSklNode[K, V]) rbSearch(x *xNode[V], fn func(*xNode[V]) int64) *xNode[V] {
	if x == nil {
		return nil
	}

	for aux := x; aux != nil; {
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

func (node *xConcSklNode[K, V]) rbDFS(action func(idx int64, color color, val V) bool) {
	size := atomic.LoadInt64(&node.count)
	aux := node.root
	if size < 0 || aux == nil {
		return
	}

	stack := make([]*xNode[V], 0, size>>1)
	defer func() {
		clear(stack)
	}()

	for ; !aux.isNilLeaf(); aux = aux.left {
		stack = append(stack, aux)
	}

	idx := int64(0)
	size = int64(len(stack))
	for size > 0 {
		if aux = stack[size-1]; !action(idx, aux.color, *aux.vptr) {
			return
		}
		idx++
		stack = stack[:size-1]
		if aux.right != nil {
			for aux = aux.right; aux != nil; aux = aux.left {
				stack = append(stack, aux)
			}
		}
		size = int64(len(stack))
	}
}

func (node *xConcSklNode[K, V]) rbRelease() {
	size := atomic.LoadInt64(&node.count)
	aux := node.root
	node.root = nil
	if size < 0 || aux == nil {
		return
	}

	stack := make([]*xNode[V], 0, size>>1)
	defer func() {
		clear(stack)
	}()

	for ; !aux.isNilLeaf(); aux = aux.left {
		stack = append(stack, aux)
	}

	size = int64(len(stack))
	for size > 0 {
		aux = stack[size-1]
		r := aux.right
		aux.right, aux.parent = nil, nil
		atomic.AddInt64(&node.count, -1)
		stack = stack[:size-1]
		if r != nil {
			for aux = r; aux != nil; aux = aux.left {
				stack = append(stack, aux)
			}
		}
		size = int64(len(stack))
	}
}

// rbtree rule validation
// References:
// https://github1s.com/minghu6/rust-minghu6/blob/master/coll_st/src/bst/rb.rs

// Inorder traversal to validate the rbtree properties.
func (node *xConcSklNode[K, V]) rbRuleValidate() error {
	size := atomic.LoadInt64(&node.count)
	aux := node.root
	if size < 0 || aux == nil {
		return nil
	}

	stack := make([]*xNode[V], 0, size>>1)
	defer func() {
		clear(stack)
	}()

	for ; !aux.isNilLeaf(); aux = aux.left {
		stack = append(stack, aux)
	}

	idx := int64(0)
	size = int64(len(stack))
	for size > 0 {
		if aux.isRed() {
			if aux.left.isRed() || aux.right.isRed() {
				return errXsklRbtreeRedViolation
			}
		}

		idx++
		stack = stack[:size-1]
		if aux.right != nil {
			for aux = aux.right; aux != nil; aux = aux.left {
				stack = append(stack, aux)
			}
		}
		size = int64(len(stack))
	}
	return nil
}

func newXConcSklNode[K infra.OrderedKey, V comparable](
	key K,
	val V,
	lvl int32,
	mu mutexImpl,
	mode xNodeMode,
	cmp SklValComparator[V],
) *xConcSklNode[K, V] {
	node := &xConcSklNode[K, V]{
		key:   key,
		level: uint32(lvl),
		mu:    mutexFactory(mu),
		vcmp:  cmp,
	}
	node.indices = newXConcSklIndices[K, V](lvl)
	node.flags.setBitsAs(xNodeModeFlagBits, uint32(mode))
	switch mode {
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
		panic("[x-conc-skl] unknown x-node type")
	}
	node.count = 1
	return node
}

func newXConcSklHead[K infra.OrderedKey, V comparable](e mutexImpl, mode xNodeMode) *xConcSklNode[K, V] {
	head := &xConcSklNode[K, V]{
		key:   *new(K),
		level: xSkipListMaxLevel,
		mu:    mutexFactory(e),
	}
	head.flags.atomicSet(nodeIsHeadFlagBit | nodeInsertedFlagBit)
	head.flags.setBitsAs(xNodeModeFlagBits, uint32(mode))
	head.indices = newXConcSklIndices[K, V](xSkipListMaxLevel)
	return head
}

func unlockNodes[K infra.OrderedKey, V comparable](version uint64, num int32, nodes ...*xConcSklNode[K, V]) {
	var prev *xConcSklNode[K, V]
	for i := num; i >= 0; i-- {
		if nodes[i] != prev {
			nodes[i].mu.unlock(version)
			prev = nodes[i]
		}
	}
}
