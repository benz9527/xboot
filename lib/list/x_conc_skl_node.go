package list

import (
	"errors"
	"log/slog"
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

func (n *xNode[V]) isRed() bool {
	return !n.isNilLeaf() && n.color == red
}

func (n *xNode[V]) isBlack() bool {
	return n.isNilLeaf() || n.color == black
}

func (n *xNode[V]) isNilLeaf() bool {
	return n == nil || (n.parent == nil && n.left == nil && n.right == nil)
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

type xConcSklNode[K infra.OrderedKey, V comparable] struct {
	// If it is unique v-node type store value directly.
	// Otherwise, it is a sentinel node.
	root        *xNode[V]
	nilLeafNode *xNode[V] // Only for rbtree
	key         K
	vcmp        SkipListValueComparator[V]
	indexes     xConcSklIndices[K, V]
	mu          segmentedMutex
	flags       flagBits
	count       int64
	level       uint32
}

func (node *xConcSklNode[K, V]) storeVal(ver uint64, val V) (isAppend bool, err error) {
	typ := vNodeType(node.flags.atomicLoadBits(vNodeTypeBits))
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

func (node *xConcSklNode[K, V]) loadVNode() *xNode[V] {
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

type vNodeDirection uint8

const (
	left vNodeDirection = iota
	right
	root
)

//	 |                         |
//	 N                         S
//	/ \     leftRotate(N)     / \
//
// L   S    ============>    N   R
//
//	 / \                   / \
//	M   R                 L   M
func (node *xConcSklNode[K, V]) rbtreeLeftRotate(vnx *xNode[V]) {
	vny := vnx.right
	vnx.right = vny.left

	if vny.left != node.nilLeafNode {
		vny.left.parent = vnx
	}

	vny.parent = vnx.parent
	if vnx.parent == node.nilLeafNode {
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
func (node *xConcSklNode[K, V]) rbtreeRightRotate(vnx *xNode[V]) {
	vny := vnx.left
	vnx.left = vny.right

	if vny.right != node.nilLeafNode {
		vny.right.parent = vnx
	}

	vny.parent = vnx.parent
	if vnx.parent == node.nilLeafNode {
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

func (node *xConcSklNode[K, V]) rbtreeInsertIfNotPresent(val V) {
	var (
		vnx, vny = node.root, node.nilLeafNode
	)

	// vnx play as the next level node detector.
	// vny store the current node found by dichotomy.
	for vnx != node.nilLeafNode {
		vny = vnx
		// Iterating to find insert position.
		// O(logN)
		res := node.vcmp(val, *vnx.vptr)
		if res < 0 {
			vnx = vnx.left
		} else if res > 0 {
			vnx = vnx.right
		} else /* res == 0 */ {
			// Replace by default
			vnx.vptr = &val
			return
		}
	}
	node.rbtreeInsert(vny, val)
}

func (node *xConcSklNode[K, V]) rbtreeInsert(vn *xNode[V], val V) {

	//nvn := &xNode[V]{
	//	vptr:    &val,
	//	color:  red,
	//	parent: vny,
	//	left:   node.nilLeafNode,
	//	right:  node.nilLeafNode,
	//}
	//
	//if node.rbtreeIsRoot(nvn) {
	//	node.root = nvn
	//	nvn.color = black
	//	atomic.AddInt64(&node.count, 1)
	//	return
	//} else if res := node.vcmp(val, *vny.val); res < 0 {
	//	if vny.left == node.nilLeafNode {
	//		vny.left = nvn
	//	} else {
	//
	//	}
	//} else /* res > 0 */ {
	//	vny.right = nvn
	//}
	//
	//node.rbtreePostInsertBalance(nvn)
	//atomic.AddInt64(&node.count, 1)
}

// New node color is red by default.
// Color adjust from the bottom-up.
func (node *xConcSklNode[K, V]) rbtreePostInsertBalance(nvn *xNode[V]) {
	var aux *xNode[V] = nil
	for nvn.parent.color == red {
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

func (node *xConcSklNode[K, V]) rbtreeMinimum(vn *xNode[V]) *xNode[V] {
	aux := vn
	for aux.left != node.nilLeafNode {
		aux = aux.left
	}
	return aux
}

func (node *xConcSklNode[K, V]) rbtreeMaximum(vn *xNode[V]) *xNode[V] {
	aux := vn
	for aux.right != node.nilLeafNode {
		aux = aux.right
	}
	return aux
}

func (node *xConcSklNode[K, V]) rbtreeIsRoot(vn *xNode[V]) bool {
	return vn.parent == node.nilLeafNode || vn == node.root
}

// The successor node of the current node is its next node in sorted order.
func (node *xConcSklNode[K, V]) rbtreeSucc(vn *xNode[V]) *xNode[V] {
	aux := vn
	if aux.right != node.nilLeafNode {
		return node.rbtreeMinimum(aux.right)
	}

	aux = vn.parent
	// Backtrack to father node that is the vn's successor.
	for aux != node.nilLeafNode && vn == aux.right {
		vn = aux
		aux = aux.parent
	}
	return aux
}

// The predecessor node of the current node is its previous node in sorted order
func (node *xConcSklNode[K, V]) rbtreePred(vn *xNode[V]) *xNode[V] {
	aux := vn
	if aux.left != node.nilLeafNode {
		return node.rbtreeMaximum(aux.left)
	}

	aux = vn.parent
	// Backtrack to father node that is the vn's predecessor.
	for aux != node.nilLeafNode && vn == aux.left {
		vn = aux
		aux = aux.parent
	}
	return aux
}

func (node *xConcSklNode[K, V]) rbtreeSibling(vn *xNode[V]) *xNode[V] {
	if node.rbtreeIsRoot(vn) {
		return node.nilLeafNode
	} else if vn == vn.parent.left {
		return vn.parent.right
	}
	/* vn == vn.parent.right */
	return vn.parent.left
}

func (node *xConcSklNode[K, V]) rbtreeHasSibling(vn *xNode[V]) bool {
	return !node.rbtreeIsRoot(vn) && node.rbtreeSibling(vn) != node.nilLeafNode
}

func (node *xConcSklNode[K, V]) rbtreeNodeDirection(vn *xNode[V]) vNodeDirection {
	if node.rbtreeIsRoot(vn) {
		return root
	} else if vn == vn.parent.left {
		return left
	}
	/* vn == vn.parent.right */
	return right
}

func (node *xConcSklNode[K, V]) rbtreeRemove(val V) error {
	if atomic.LoadInt64(&node.count) <= 0 {
		return errors.New("empty rbtree")
	}
	vnz := node.rbtreeSearch(node.root, func(vn *xNode[V]) int64 {
		return node.vcmp(val, *vn.vptr)
	})
	if vnz == nil || vnz == node.nilLeafNode {
		return errors.New("not exists")
	}
	var vny = node.nilLeafNode

	// Found vnz is the remove target node
	// case 1: vnz is the root node of rbtree, remove directly
	if vnz.parent == node.nilLeafNode {
		node.root = node.nilLeafNode
		vnz.left = nil
		vnz.right = nil
		atomic.AddInt64(&node.count, -1)
		return nil
	}

	vny = vnz
	// case 2: vny contains 2 not nil leaf node
	if vny.left != node.nilLeafNode && vny.right != node.nilLeafNode {
		// Find the successor then swap value only
		//     |                    |
		//     N                    S
		//    / \                  / \
		//   L  ..   swap(N, S)   L  ..
		//       |   =========>       |
		//       P                    P
		//      / \                  / \
		//     S  ..                N  ..
		vny = node.rbtreeSucc(vnz)
		// Swap value only.
		vnz.vptr = vny.vptr
	}

	// case 3: vny is a leaf node.
	if vny.left == node.nilLeafNode && vny.right == node.nilLeafNode {
		if vny.color == black {
			node.rbtreeRemoveBalance(vny)
			if vny == vny.parent.left {
				vny.parent.left = node.nilLeafNode
			} else /* vny == vny.parent.right */ {
				vny.parent.right = node.nilLeafNode
			}
		} else if vny.color == red {
			// Leaf red node, remove directly.
			if vny == vny.parent.left {
				vny.parent.left = node.nilLeafNode
			} else if vny == vny.parent.right {
				vny.parent.right = node.nilLeafNode
			}
			atomic.AddInt64(&node.count, -1)
			return nil
		}
	} else /* vny.left != node.nilLeafNode || vny.right != node.nilLeafNode */ {
		// case 4: vny is not a leaf node.
		var rvn = node.nilLeafNode
		if vny.left != node.nilLeafNode {
			rvn = vny.left // Maybe a red node
		} else /* vny.right != node.nilLeafNode */ {
			rvn = vny.right // Maybe a red node
		}
		if vny.parent == node.nilLeafNode {
			// Root node of rbtree
			node.root = rvn
		} else if vny == vny.parent.left {
			vny.parent.left = rvn
			rvn.parent = vny.parent
		} else /* vny == vny.parent.right */ {
			vny.parent.right = rvn
			rvn.parent = vny.parent
		}

		if vny.color == black {
			if rvn.color == red {
				rvn.color = black
			} else {
				node.rbtreeRemoveBalance(rvn)
			}
		}
	}

	vny.parent = nil
	vny.left = nil
	vny.right = nil

	// If it is red, directly remove is okay.
	atomic.AddInt64(&node.count, -1)
	return nil
}

func (node *xConcSklNode[K, V]) rbtreeRemoveByPred(val V) (*xNode[V], error) {
	if atomic.LoadInt64(&node.count) <= 0 {
		return nil, errors.New("empty rbtree")
	}
	vnz := node.rbtreeSearch(node.root, func(vn *xNode[V]) int64 {
		return node.vcmp(val, *vn.vptr)
	})
	if vnz == nil || vnz == node.nilLeafNode {
		return nil, errors.New("not exists")
	}
	var vny = node.nilLeafNode

	// Found vnz is the remove target node
	// case 1: vnz is the root node of rbtree, remove directly
	if vnz.parent == node.nilLeafNode {
		node.root = node.nilLeafNode
		vnz.left = nil
		vnz.right = nil
		atomic.AddInt64(&node.count, -1)
		return vnz, nil
	}

	vny = vnz
	// case 2: vny contains 2 not nil leaf node
	if vny.left != node.nilLeafNode && vny.right != node.nilLeafNode {
		// Find the predecessor then swap value only
		//     |                    |
		//     N                    L
		//    / \                  / \
		//   L  ..   swap(N, L)   N  ..
		//       |   =========>       |
		//       P                    P
		//      / \                  / \
		//     S  ..                S  ..
		vny = node.rbtreePred(vnz)
		// Swap value only.
		slog.Info("before swap", "old vnz", *vnz.vptr, "new vny", *vny.vptr)
		vnz.vptr = vny.vptr
	}

	// case 3: vny is a leaf node.
	if vny.left == node.nilLeafNode && vny.right == node.nilLeafNode {
		if vny.color == black {
			node.rbtreeRemoveBalance(vny)
			if vny == vny.parent.left {
				vny.parent.left = node.nilLeafNode
			} else /* vny == vny.parent.right */ {
				vny.parent.right = node.nilLeafNode
			}
		} else if vny.color == red {
			// Leaf red node, remove directly.
			if vny == vny.parent.left {
				vny.parent.left = node.nilLeafNode
			} else if vny == vny.parent.right {
				vny.parent.right = node.nilLeafNode
			}
			atomic.AddInt64(&node.count, -1)
			return vny, nil
		}
	} else /* vny.left != node.nilLeafNode || vny.right != node.nilLeafNode */ {
		// case 4: vny is not a leaf node.
		var rvn = node.nilLeafNode
		if vny.right != node.nilLeafNode {
			rvn = vny.right // Maybe a red node
		} else /* vny.left != node.nilLeafNode */ {
			rvn = vny.right // Maybe a red node
		}
		if vny.parent == node.nilLeafNode {
			// Root node of rbtree
			node.root = rvn
		} else if vny == vny.parent.left {
			vny.parent.left = rvn
			rvn.parent = vny.parent
		} else /* vny == vny.parent.right */ {
			vny.parent.right = rvn
			rvn.parent = vny.parent
		}

		if vny.color == black {
			if rvn.color == red {
				rvn.color = black
			} else {
				node.rbtreeRemoveBalance(rvn)
			}
		}
	}

	// Unlink node
	if vny == vny.parent.left {
		vny.parent.left = node.nilLeafNode
	} else if vny == vny.parent.right {
		vny.parent.right = node.nilLeafNode
	}

	vny.parent = nil
	vny.left = nil
	vny.right = nil

	// If it is red, directly remove is okay.
	atomic.AddInt64(&node.count, -1)
	return vny, nil
}

func (node *xConcSklNode[K, V]) rbtreeRemoveBalance(vn *xNode[V]) {
	if vn.parent == node.nilLeafNode {
		// Backtrack to root node
		return
	}

	sibling := node.rbtreeSibling(vn)
	vnDir := node.rbtreeNodeDirection(vn)
	if sibling.color == red {
		// case 1: vn's sibling node is red
		if vnDir == left {
			// [] is black, <> is red
			//     |                     |
			//    [P]                   [S]
			//    / \                  /   \
			// [N]  <S>               <P>  [Sr]
			//      /  \   =======>  /  \
			//    [Sl] [Sr]        [N]  [Sl]
			// rotate father node
			node.rbtreeLeftRotate(vn.parent)
		} else if vnDir == right {
			node.rbtreeRightRotate(vn.parent)
		}
		sibling.color = black
		vn.parent.color = red
		sibling = node.rbtreeSibling(vn)
	}

	var slvn, srvn = node.nilLeafNode, node.nilLeafNode
	if vnDir == left {
		slvn = sibling.left
		srvn = sibling.right
	} else if vnDir == right {
		slvn = sibling.right
		srvn = sibling.left
	}

	// sibling must be black

	if slvn.color == black && srvn.color == black {
		if vn.parent.color == red {
			//      <P>             [P]
			//      / \             / \
			//    [N] [S]  ====>  [N] <S>
			//        / \             / \
			//     [Sl] [Sr]       [Sl] [Sr]
			sibling.color = red
			vn.parent.color = black
			return
		}
		//      [P]             [P]
		//      / \             / \
		//    [N] [S]  ====>  [N] <S>
		//        / \             / \
		//      [C] [D]         [C] [D]
		sibling.color = red
		node.rbtreeRemoveBalance(vn.parent)
		return
	} else {
		if slvn != node.nilLeafNode && slvn.color == red {
			if vnDir == left {
				//                            {P}                {P}
				//      {P}                   / \                / \
				//      / \    r-rotate(S)  [N] <Sl>   repaint  [N] [Sl]
				//    [N] [S]  ==========>        \    ======>       \
				//        / \                     [S]                <S>
				//     <Sl> [Sr]                    \                  \
				//                                  [Sr]                [Sr]
				node.rbtreeRightRotate(sibling)
			} else if vnDir == right {
				node.rbtreeLeftRotate(sibling)
			}
			slvn.color = black
			sibling.color = red
			sibling = node.rbtreeSibling(vn)

			if vnDir == left {
				slvn = sibling.left
				srvn = sibling.right
			} else if vnDir == right {
				slvn = sibling.right
				srvn = sibling.left
			}
		}

		if vnDir == left {
			//      {P}                   [S]
			//      / \    l-rotate(P)    / \
			//    [N] [S]  ==========>  {P} <D>
			//        / \               / \
			//      [C] <D>           [N] [C]
			node.rbtreeLeftRotate(vn.parent)
		} else if vnDir == right {
			node.rbtreeRightRotate(vn.parent)
		}
		sibling.color = vn.parent.color
		vn.parent.color = black
		if srvn != node.nilLeafNode {
			srvn.color = black
		}
		return
	}
}

func (node *xConcSklNode[K, V]) rbtreeSearch(vn *xNode[V], fn func(*xNode[V]) int64) *xNode[V] {
	if vn == nil || vn == node.nilLeafNode {
		return node.nilLeafNode
	}
	aux := vn
	for aux != node.nilLeafNode {
		res := fn(aux)
		if res == 0 {
			return aux
		} else if res > 0 {
			aux = aux.right
		} else {
			aux = aux.left
		}
	}
	return node.nilLeafNode
}

func (node *xConcSklNode[K, V]) rbtreePreorderTraversal(fn func(idx int64, color color, val V) bool) {
	size := atomic.LoadInt64(&node.count)
	aux := node.root
	if size < 0 || aux == nil || aux == node.nilLeafNode {
		return
	}
	stack := make([]*xNode[V], 0, size>>1)
	defer func() {
		clear(stack)
	}()
	for aux != node.nilLeafNode {
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
		if aux.right != node.nilLeafNode {
			aux = aux.right
			for aux != node.nilLeafNode {
				stack = append(stack, aux)
				aux = aux.left
			}
		}
		size = int64(len(stack))
	}
}

func (node *xConcSklNode[K, V]) rbtreeInorderTraversal(fn func(idx int64, color color, val V) bool) {

}

func (node *xConcSklNode[K, V]) rbtreePostorderTraversal(fn func(idx int64, color color, val V) bool) {

}

func newXConcSkipListNode[K infra.OrderedKey, V comparable](
	key K,
	val V,
	lvl int32,
	mu mutexImpl,
	typ vNodeType,
	cmp SkipListValueComparator[V],
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
		node.nilLeafNode = &xNode[V]{
			color: black,
		}
		node.root = node.nilLeafNode
		node.rbtreeInsertIfNotPresent(val)
	default:
		panic("unknown v-node type")
	}
	node.count = 1
	return node
}

func newXConcSklHead[K infra.OrderedKey, V comparable](e mutexImpl, typ vNodeType) *xConcSklNode[K, V] {
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
