// Auto generated code; Do NOT EDIT!
// Author benzheng2121@126.com

package list

import (
	"github.com/benz9527/xboot/lib/infra"
)

func genXConcSklUniqueNode[K infra.OrderedKey, V any](
	key K,
	val V,
	lvl int32,
) *xConcSklNode[K, V] {
	switch lvl {
	case 1: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [1]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 2: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [2]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 3: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [3]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 4: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [4]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 5: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [5]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 6: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [6]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 7: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [7]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 8: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [8]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 9: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [9]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 10: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [10]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 11: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [11]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 12: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [12]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 13: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [13]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 14: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [14]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 15: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [15]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 16: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [16]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 17: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [17]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 18: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [18]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 19: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [19]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 20: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [20]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 21: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [21]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 22: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [22]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 23: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [23]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 24: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [24]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 25: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [25]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 26: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [26]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 27: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [27]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 28: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [28]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 29: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [29]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 30: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [30]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 31: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [31]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	case 32: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [32]*xConcSklNode[K, V]
		xN      xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(unique))
	  n.xN.vptr = &val
	  n.node.root = &n.xN
	  n.node.count = 1
	  return &n.node
	default:
	}
	panic("unable to generate ")
}

func genXConcSklLinkedListNode[K infra.OrderedKey, V any](
	key K,
	val V,
	lvl int32,
) *xConcSklNode[K, V] {
	switch lvl {
	case 1: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [1]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 2: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [2]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 3: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [3]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 4: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [4]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 5: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [5]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 6: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [6]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 7: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [7]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 8: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [8]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 9: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [9]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 10: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [10]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 11: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [11]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 12: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [12]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 13: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [13]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 14: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [14]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 15: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [15]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 16: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [16]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 17: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [17]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 18: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [18]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 19: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [19]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 20: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [20]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 21: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [21]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 22: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [22]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 23: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [23]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 24: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [24]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 25: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [25]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 26: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [26]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 27: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [27]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 28: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [28]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 29: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [29]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 30: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [30]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 31: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [31]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	case 32: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [32]*xConcSklNode[K, V]
		xN      [2]xNode[V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(linkedList))
	  n.xN[1].vptr = &val
	  n.node.root = &n.xN[0]
	  n.node.root.parent = &n.xN[1]
	  n.node.count = 1
	  return &n.node
	default:
	}
	panic("unable to generate ")
}

func genXConcSklRbtreeNode[K infra.OrderedKey, V any](
	key K,
	val V,
	vcmp SklValComparator[V],
	lvl int32,
) *xConcSklNode[K, V] {
	switch lvl {
	case 1: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [1]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 2: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [2]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 3: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [3]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 4: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [4]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 5: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [5]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 6: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [6]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 7: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [7]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 8: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [8]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 9: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [9]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 10: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [10]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 11: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [11]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 12: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [12]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 13: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [13]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 14: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [14]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 15: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [15]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 16: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [16]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 17: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [17]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 18: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [18]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 19: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [19]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 20: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [20]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 21: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [21]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 22: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [22]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 23: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [23]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 24: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [24]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 25: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [25]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 26: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [26]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 27: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [27]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 28: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [28]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 29: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [29]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 30: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [30]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 31: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [31]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	case 32: 
	  n := struct {
		node    xConcSklNode[K, V]
		indices [32]*xConcSklNode[K, V]
	  }{}
	  n.node.indices = n.indices[:]
	  n.node.key = key
	  n.node.level = uint32(lvl)
	  n.node.flags = setBitsAs(0, xNodeModeFlagBits, uint32(rbtree))
	  n.node.rbInsert(val, vcmp)
	  n.node.count = 1
	  return &n.node
	default:
	}
	panic("unable to generate ")
}
