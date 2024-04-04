package tree

import (
	"errors"

	"github.com/benz9527/xboot/lib/infra"
)

func isBlack[K infra.OrderedKey, V any](node RBNode[K, V]) bool {
	return isNilLeaf[K, V](node) || node.Color() == Black
}

func isRed[K infra.OrderedKey, V any](node RBNode[K, V]) bool {
	return !isNilLeaf[K, V](node) && node.Color() == Red
}

func isNilLeaf[K infra.OrderedKey, V any](node RBNode[K, V]) bool {
	return node == nil || (!node.HasKeyVal() && node.Parent() == nil && node.Left() == nil && node.Right() == nil)
}

func isRoot[K infra.OrderedKey, V any](node RBNode[K, V]) bool {
	return node != nil && node.Parent() == nil
}

func blackDepthTo[K infra.OrderedKey, V any](target, to RBNode[K, V]) int {
	depth := 0
	for aux := target; aux != to; aux = aux.Parent() {
		if isBlack[K, V](aux) {
			depth++
		}
	}
	return depth
}

// rbtree rule validation utilities.

// References:
// https://github1s.com/minghu6/rust-minghu6/blob/master/coll_st/src/bst/rb.rs

// Inorder traversal to validate the rbtree properties.
func RedViolationValidate[K infra.OrderedKey, V any](tree RBTree[K, V]) error {
	size := tree.Len()
	var aux RBNode[K, V] = tree.Root()
	if size < 0 || aux == nil {
		return nil
	}

	stack := make([]RBNode[K, V], 0, size>>1)
	defer func() {
		clear(stack)
	}()

	for ; !isNilLeaf[K, V](aux); aux = aux.Left() {
		stack = append(stack, aux)
	}

	for size = int64(len(stack)); size > 0; size = int64(len(stack)) {
		if aux = stack[size-1]; isRed[K, V](aux) {
			if (!isRoot[K, V](aux.Parent()) && isRed[K, V](aux.Parent())) ||
				(isRed[K, V](aux.Left()) || isRed[K, V](aux.Right())) {
				return errors.New("rbtree red violation")
			}
		}

		stack = stack[:size-1]
		if aux.Right() != nil {
			for aux = aux.Right(); aux != nil; aux = aux.Left() {
				stack = append(stack, aux)
			}
		}
	}
	return nil
}

// BFS traversal to load all leaves.
func bfsLeaves[K infra.OrderedKey, V any](tree RBTree[K, V]) []RBNode[K, V] {
	size := tree.Len()
	var aux RBNode[K, V] = tree.Root()
	if size < 0 || isNilLeaf[K, V](aux) {
		return nil
	}

	leaves := make([]RBNode[K, V], 0, size>>1+1)
	stack := make([]RBNode[K, V], 0, size>>1)
	defer func() {
		clear(stack)
	}()
	stack = append(stack, aux)

	for len(stack) > 0 {
		aux = stack[0]
		l, r := aux.Left(), aux.Right()
		if /* nil leaves, keep one */ isNilLeaf[K, V](l) || isNilLeaf[K, V](r) {
			leaves = append(leaves, aux)
		}
		if !isNilLeaf[K, V](l) {
			stack = append(stack, l)
		}
		if !isNilLeaf[K, V](r) {
			stack = append(stack, r)
		}
		stack = stack[1:]
	}
	return leaves
}

/*
<X> is a RED node.
[X] is a BLACK node (or NIL).

	        [13]
			/  \
		 <8>    [15]
		 / \    /  \
	  [6] [11] [14] [17]
	  /              /
	<1>            [16]

2-3-4 tree like:

	       <8> --- [13] --- <15>
		  /  \             /    \
		 /    \           /      \
	  <1>-[6][11]      [14] <16>-[17]

Each leaf node to root node black depth are equal.
*/
func BlackViolationValidate[K infra.OrderedKey, V any](tree RBTree[K, V]) error {
	leaves := bfsLeaves[K, V](tree)
	if leaves == nil {
		return nil
	}

	blackDepth := blackDepthTo[K, V](leaves[0], tree.Root())
	for i := 1; i < len(leaves); i++ {
		if blackDepthTo[K, V](leaves[i], tree.Root()) != blackDepth {
			return errors.New("rbtree black violation")
		}
	}
	return nil
}
