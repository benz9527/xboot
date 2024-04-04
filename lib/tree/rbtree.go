package tree

import (
	"errors"
	"sync/atomic"

	"github.com/benz9527/xboot/lib/infra"
)

type rbNode[K infra.OrderedKey, V any] struct {
	parent *rbNode[K, V]
	left   *rbNode[K, V]
	right  *rbNode[K, V]
	key    K
	val    V
	color  RBColor
	hasKV  bool
}

func (node *rbNode[K, V]) Color() RBColor {
	return node.color
}

func (node *rbNode[K, V]) Key() K {
	return node.key
}

func (node *rbNode[K, V]) Val() V {
	return node.val
}

func (node *rbNode[K, V]) HasKeyVal() bool {
	if node == nil {
		return false
	}
	return node.hasKV
}

func (node *rbNode[K, V]) Left() RBNode[K, V] {
	if node == nil || node.left == nil {
		return nil
	}
	return node.left
}

func (node *rbNode[K, V]) Parent() RBNode[K, V] {
	if node == nil || node.parent == nil {
		return nil
	}
	return node.parent
}

func (node *rbNode[K, V]) Right() RBNode[K, V] {
	if node == nil || node.right == nil {
		return nil
	}
	return node.right
}

func (node *rbNode[K, V]) isNilLeaf() bool {
	return isNilLeaf[K, V](node)
}

func (node *rbNode[K, V]) isRed() bool {
	return isRed[K, V](node)
}

func (node *rbNode[K, V]) isBlack() bool {
	return isBlack[K, V](node)
}

func (node *rbNode[K, V]) isRoot() bool {
	return isRoot[K, V](node)
}

func (node *rbNode[K, V]) isLeaf() bool {
	return node != nil && node.parent != nil && node.left.isNilLeaf() && node.right.isNilLeaf()
}

func (node *rbNode[K, V]) Direction() RBDirection {
	if node.isNilLeaf() {
		// impossible run to here
		panic( /* debug assertion */ "[rbtree] nil leaf node without direction")
	}

	if node.isRoot() {
		return Root
	}
	if node == node.parent.left {
		return Left
	}
	return Right
}

func (node *rbNode[K, V]) sibling() *rbNode[K, V] {
	dir := node.Direction()
	switch dir {
	case Left:
		return node.parent.right
	case Right:
		return node.parent.left
	default:

	}
	return nil
}

func (node *rbNode[K, V]) hasSibling() bool {
	return !node.isRoot() && node.sibling() != nil
}

func (node *rbNode[K, V]) uncle() *rbNode[K, V] {
	return node.parent.sibling()
}

func (node *rbNode[K, V]) hasUncle() bool {
	return !node.isRoot() && node.parent.hasSibling()
}

func (node *rbNode[K, V]) grandpa() *rbNode[K, V] {
	return node.parent.parent
}

func (node *rbNode[K, V]) hasGrandpa() bool {
	return !node.isRoot() && node.parent.parent != nil
}

func (node *rbNode[K, V]) fixLink() {
	if node.left != nil {
		node.left.parent = node
	}
	if node.right != nil {
		node.right.parent = node
	}
}

func (node *rbNode[K, V]) minimum() *rbNode[K, V] {
	aux := node
	for ; aux != nil && aux.left != nil; aux = aux.left {
	}
	return aux
}

func (node *rbNode[K, V]) maximum() *rbNode[K, V] {
	aux := node
	for ; aux != nil && aux.right != nil; aux = aux.right {
	}
	return aux
}

// The pred node of the current node is its previous node in sorted order
func (node *rbNode[K, V]) pred() *rbNode[K, V] {
	x := node
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
func (node *rbNode[K, V]) succ() *rbNode[K, V] {
	x := node
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

type rbTree[K infra.OrderedKey, V any] struct {
	root           *rbNode[K, V]
	count          int64
	isDesc         bool
	isRmBorrowSucc bool
}

func (tree *rbTree[K, V]) keyCompare(k1, k2 K) int64 {
	if k1 == k2 {
		return 0
	} else if k1 < k2 {
		if !tree.isDesc {
			return -1
		}
		return 1
	} else {
		if !tree.isDesc {
			return 1
		}
		return -1
	}
}

func (tree *rbTree[K, V]) Len() int64 {
	return atomic.LoadInt64(&tree.count)
}

func (tree *rbTree[K, V]) Root() RBNode[K, V] {
	return tree.root
}

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
func (tree *rbTree[K, V]) leftRotate(x *rbNode[K, V]) {
	if x == nil || x.right.isNilLeaf() {
		// impossible run to here
		panic( /* debug assertion */ "[rbtree] left rotate node x is nil or x.right is nil")
	}

	p, y := x.parent, x.right
	dir := x.Direction()
	x.right, y.left = y.left, x

	x.fixLink()
	y.fixLink()

	switch dir {
	case Root:
		tree.root = y
	case Left:
		p.left = y
	case Right:
		p.right = y
	default:
		// impossible run to here
		panic( /* debug assertion */ "[rbtree] unknown node direction to left-rotate")
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
func (tree *rbTree[K, V]) rightRotate(x *rbNode[K, V]) {
	if x == nil || x.left.isNilLeaf() {
		// impossible run to here
		panic( /* debug assertion */ "[rbtree] right rotate node x is nil or x.right is nil")
	}

	p, y := x.parent, x.left
	dir := x.Direction()
	x.left, y.right = y.right, x

	x.fixLink()
	y.fixLink()

	switch dir {
	case Root:
		tree.root = y
	case Left:
		p.left = y
	case Right:
		p.right = y
	default:
		// impossible run to here
		panic( /* debug assertion */ "[rbtree] unknown node direction to right-rotate")
	}
	y.parent = p
}

// i1: Empty rbtree, insert directly, but root node is painted to black.
func (tree *rbTree[K, V]) Insert(key K, val V, ifNotPresent ...bool) (err error) {
	if /* i1 */ tree.root.isNilLeaf() {
		tree.root = &rbNode[K, V]{
			key:   key,
			val:   val,
			hasKV: true,
		}
		atomic.AddInt64(&tree.count, 1)
		return nil
	}

	var x, y *rbNode[K, V] = tree.root, nil
	for !x.isNilLeaf() {
		y = x
		res := tree.keyCompare(key, x.key)
		if /* equal */ res == 0 {
			break
		} else /* less */ if res < 0 {
			x = x.left
		} else /* greater */ {
			x = x.right
		}
	}

	if y.isNilLeaf() {
		// impossible run to here
		panic( /* debug assertion */ "[rbtree] insert a new value into nil node")
	}

	var z *rbNode[K, V]
	res := tree.keyCompare(key, y.key)
	if /* equal */ res == 0 {
		if /* disabled */ ifNotPresent[0] {
			return errors.New("[rbtree] replace disabled")
		}
		y.val = val
		return nil
	} else /* less */ if res < 0 {
		z = &rbNode[K, V]{
			key:    key,
			val:    val,
			color:  Red,
			parent: y,
			hasKV:  true,
		}
		y.left = z
	} else /* greater */ {
		z = &rbNode[K, V]{
			key:    key,
			val:    val,
			color:  Red,
			parent: y,
			hasKV:  true,
		}
		y.right = z
	}

	atomic.AddInt64(&tree.count, 1)
	tree.insertRebalance(z)
	return nil
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
func (tree *rbTree[K, V]) insertRebalance(x *rbNode[K, V]) {
	for !x.isNilLeaf() {
		if x.isRoot() {
			if x.isRed() {
				x.color = Black
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
				x.parent.color = Black
			}
		}

		if /* im3 */ x.hasUncle() && x.uncle().isRed() {
			x.parent.color = Black
			x.uncle().color = Black
			gp := x.grandpa()
			gp.color = Red
			x = gp
			continue
		} else {
			if !x.hasUncle() || x.uncle().isBlack() {
				dir := x.Direction()
				if /* im4 */ dir != x.parent.Direction() {
					p := x.parent
					switch dir {
					case Left:
						tree.rightRotate(p)
					case Right:
						tree.leftRotate(p)
					default:
						// impossible run to here
						panic( /* debug assertion */ "[x-conc-skl] rbtree insert violate (im4)")
					}
					x = p // enter im5 to fix
				}

				switch /* im5 */ dir = x.parent.Direction(); dir {
				case Left:
					tree.rightRotate(x.grandpa())
				case Right:
					tree.leftRotate(x.grandpa())
				default:
					// impossible run to here
					panic( /* debug assertion */ "[x-conc-skl] rbtree insert violate (im5)")
				}

				x.parent.color = Black
				x.sibling().color = Red
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
func (tree *rbTree[K, V]) removeNode(z *rbNode[K, V]) (res *rbNode[K, V], err error) {
	if /* r1 */ atomic.LoadInt64(&tree.count) == 1 && z.isRoot() {
		tree.root = nil
		z.left = nil
		z.right = nil
		return z, nil
	}

	res = &rbNode[K, V]{
		val: z.val,
		key: z.key,
	}

	y := z
	if /* r2 */ !y.left.isNilLeaf() && !y.right.isNilLeaf() {
		if tree.isRmBorrowSucc {
			y = z.succ() // enter r3-r4
		} else {
			y = z.pred() // enter r3-r4
		}
		// Swap key & value.
		z.key, z.val = y.key, y.val
		z.hasKV = true
	}

	if /* r3 */ y.isLeaf() {
		if /* r3 (1) */ y.isRed() {
			switch dir := y.Direction(); dir {
			case Left:
				y.parent.left = nil
			case Right:
				y.parent.right = nil
			default:
				// impossible run to here
				panic( /* debug assertion */ "[rbtree] y should be a leaf node, violate (r3-1)")
			}
			return res, nil
		} else /* r3 (2) */ {
			tree.removeRebalance(y)
		}
	} else /* r4 */ {
		var replace *rbNode[K, V]
		if !y.right.isNilLeaf() {
			replace = y.right
		} else if !y.left.isNilLeaf() {
			replace = y.left
		}

		if replace == nil {
			// impossible run to here
			panic( /* debug assertion */ "[rbtree] remove a leaf node without child, violate (r4)")
		}

		switch dir := y.Direction(); dir {
		case Root:
			tree.root = replace
			tree.root.parent = nil
		case Left:
			y.parent.left = replace
			replace.parent = y.parent
		case Right:
			y.parent.right = replace
			replace.parent = y.parent
		default:
			// impossible run to here
			panic( /* debug assertion */ "[x-conc-skl] rbtree impossible run to here")
		}

		if y.isBlack() {
			if replace.isRed() {
				replace.color = Black
			} else {
				tree.removeRebalance(replace)
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
	y.hasKV = false

	return res, nil
}

func (tree *rbTree[K, V]) Remove(key K) (RBNode[K, V], error) {
	if atomic.LoadInt64(&tree.count) <= 0 {
		return nil, errors.New("[rbtree] empty element to remove")
	}
	z := tree.Search(tree.root, func(node RBNode[K, V]) int64 {
		return tree.keyCompare(key, node.Key())
	})
	if z == nil {
		return nil, errors.New("[rbtree] key not found")
	}
	defer func() {
		atomic.AddInt64(&tree.count, -1)
	}()

	return tree.removeNode(z.(*rbNode[K, V]))
}

func (tree *rbTree[K, V]) RemoveMin() (RBNode[K, V], error) {
	if atomic.LoadInt64(&tree.count) <= 0 {
		return nil, errors.New("[rbtree] key not found")
	}
	_min := tree.root.minimum()
	if _min.isNilLeaf() {
		return nil, errors.New("[rbtree] key not found")
	}
	defer func() {
		atomic.AddInt64(&tree.count, -1)
	}()
	return tree.removeNode(_min)
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
func (tree *rbTree[K, V]) removeRebalance(x *rbNode[K, V]) {
	for {
		if x.isRoot() {
			return
		}

		sibling := x.sibling()
		dir := x.Direction()
		if /* rm1 */ sibling.isRed() {
			switch dir {
			case Left:
				tree.leftRotate(x.parent)
			case Right:
				tree.rightRotate(x.parent)
			default:
				// impossible run to here
				panic( /* debug assertion */ "[rbtree] remove violate (rm1)")
			}
			sibling.color = Black
			x.parent.color = Red // ready to enter rm2
			sibling = x.sibling()
		}

		var sc, sd *rbNode[K, V]
		switch /* rm2 */ dir {
		case Left:
			sc, sd = sibling.left, sibling.right
		case Right:
			sc, sd = sibling.right, sibling.left
		default:
			// impossible run to here
			panic( /* debug assertion */ "[x-conc-skl] rbtree remove violate (rm2)")
		}

		if sc.isBlack() && sd.isBlack() {
			if /* rm2 */ x.parent.isRed() {
				sibling.color = Red
				x.parent.color = Black
				break
			} else /* rm3 */ {
				sibling.color = Red
				x = x.parent
				continue
			}
		} else {
			if /* rm 4 */ !sc.isNilLeaf() && sc.isRed() {
				switch dir {
				case Left:
					tree.rightRotate(sibling)
				case Right:
					tree.leftRotate(sibling)
				default:
					// impossible run to here
					panic( /* debug assertion */ "[x-conc-skl] rbtree remove violate (rm4)")
				}
				sc.color = Black
				sibling.color = Red
				sibling = x.sibling()
				switch dir {
				case Left:
					sd = sibling.right
				case Right:
					sd = sibling.left
				default:
					// impossible run to here
					panic( /* debug assertion */ "[x-conc-skl] rbtree remove violate (rm4)")
				}
			}

			switch /* rm5 */ dir {
			case Left:
				tree.leftRotate(x.parent)
			case Right:
				tree.rightRotate(x.parent)
			default:
				// impossible run to here
				panic( /* debug assertion */ "[x-conc-skl] rbtree remove violate (rm5)")
			}
			sibling.color = x.parent.color
			x.parent.color = Black
			if !sd.isNilLeaf() {
				sd.color = Black
			}
			break
		}
	}
}

func (tree *rbTree[K, V]) Search(x RBNode[K, V], fn func(RBNode[K, V]) int64) RBNode[K, V] {
	if x == nil {
		return nil
	}

	for aux := x; aux != nil; {
		res := fn(aux)
		if res == 0 {
			return aux
		} else if res > 0 {
			aux = aux.Right()
		} else {
			aux = aux.Left()
		}
	}
	return nil
}

// Inorder traversal to implement the DFS.
func (tree *rbTree[K, V]) Foreach(action func(idx int64, color RBColor, key K, val V) bool) {
	size := atomic.LoadInt64(&tree.count)
	aux := tree.root
	if size < 0 || aux == nil {
		return
	}

	stack := make([]*rbNode[K, V], 0, size>>1)
	defer func() {
		clear(stack)
	}()

	for ; !aux.isNilLeaf(); aux = aux.left {
		stack = append(stack, aux)
	}

	idx := int64(0)
	for size = int64(len(stack)); size > 0; size = int64(len(stack)) {
		if aux = stack[size-1]; !action(idx, aux.color, aux.key, aux.val) {
			return
		}
		idx++
		stack = stack[:size-1]
		if aux.right != nil {
			for aux = aux.right; aux != nil; aux = aux.left {
				stack = append(stack, aux)
			}
		}
	}
}

func (tree *rbTree[K, V]) Release() {
	size := atomic.LoadInt64(&tree.count)
	aux := tree.root
	tree.root = nil
	if size < 0 || aux == nil {
		return
	}

	stack := make([]*rbNode[K, V], 0, size>>1)
	defer func() {
		clear(stack)
	}()

	for ; !aux.isNilLeaf(); aux = aux.left {
		stack = append(stack, aux)
	}

	for size = int64(len(stack)); size > 0; size = int64(len(stack)) {
		aux = stack[size-1]
		r := aux.right
		aux.right, aux.parent = nil, nil
		atomic.AddInt64(&tree.count, -1)
		stack = stack[:size-1]
		if r != nil {
			for aux = r; aux != nil; aux = aux.left {
				stack = append(stack, aux)
			}
		}
	}
}

type RBTreeOpt[K infra.OrderedKey, V any] func(*rbTree[K, V])

func WithRBTreeDesc[K infra.OrderedKey, V any]() RBTreeOpt[K, V] {
	return func(tree *rbTree[K, V]) {
		tree.isDesc = true
	}
}

func WithRBTreeRemoveBorrowSucc[K infra.OrderedKey, V any]() RBTreeOpt[K, V] {
	return func(tree *rbTree[K, V]) {
		tree.isRmBorrowSucc = true
	}
}

func NewRBTree[K infra.OrderedKey, V any](opts ...RBTreeOpt[K, V]) RBTree[K, V] {
	tree := &rbTree[K, V]{
		count:          0,
		isDesc:         false,
		isRmBorrowSucc: false,
	}

	for _, o := range opts {
		o(tree)
	}
	return tree
}
