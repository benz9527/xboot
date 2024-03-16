package list

import (
	"github.com/benz9527/xboot/lib/id"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
)

type vNode[W SkipListWeight, O HashObject] struct {
	weight *atomic.Pointer[W]
	object *atomic.Pointer[O]
	next   *atomic.Pointer[vNode[W, O]]
}

func TestAtomicPointerCAS(t *testing.T) {
	a := &vNode[uint8, *xSkipListObject]{
		weight: &atomic.Pointer[uint8]{},
		object: &atomic.Pointer[*xSkipListObject]{},
		next:   nil,
	}
	w1 := uint8(1)
	o1 := &xSkipListObject{
		id: "1",
	}
	a.weight.Store(&w1)
	a.object.Store(&o1)

	b := a

	w2 := uint8(2)
	o2 := &xSkipListObject{
		id: "2",
	}
	c := &vNode[uint8, *xSkipListObject]{
		weight: &atomic.Pointer[uint8]{},
		object: &atomic.Pointer[*xSkipListObject]{},
		next:   nil,
	}
	c.weight.Store(&w2)
	c.object.Store(&o2)

	ptr := atomic.Pointer[vNode[uint8, *xSkipListObject]]{}
	ptr.Store(a)
	res := ptr.CompareAndSwap(b, c)
	require.True(t, res)
	require.Equal(t, c, ptr.Load())
}

func TestUnsafePointerCAS(t *testing.T) {
	type obj struct {
		id int
	}
	a := &obj{
		id: 1,
	}
	b := a
	c := &obj{
		id: 2,
	}

	res := atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&a)), unsafe.Pointer(b), unsafe.Pointer(c))
	require.True(t, res)
	require.Equal(t, c, a)
}

func TestUnsafePointerCAS_ConcurrentMemVisibility(t *testing.T) {
	runtime.GOMAXPROCS(4)
	type obj struct {
		id string
	}
	a := &obj{
		id: "1",
	}
	b := a
	c := &obj{
		id: "2",
	}

	logC := make(chan string, 1000)

	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		for i := 0; i < 200; i++ {
			val := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&a)))
			log := "cas load a vptr, obj: " + (*obj)(val).id + ", id: 1"
			runtime.Gosched()
			logC <- log
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < 200; i++ {
			val := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&a)))
			log := "cas load a vptr, obj: " + (*obj)(val).id + ", id: 2"
			runtime.Gosched()
			logC <- log
		}
		wg.Done()
	}()
	go func() {
		swapped := false
		for i := 0; i < 50; i++ {
			ms := cryptoRandUint32() % 11
			if ms == 5 && !swapped {
				log := "starting to cas obj to c, id: 1, loop: " + strconv.Itoa(i)
				logC <- log
				swapped = atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&a)), unsafe.Pointer(b), unsafe.Pointer(c))
				if swapped != true {
					log = "cas result is false, id: 1"
					logC <- log
				} else {
					log = "cas result is ok, id: 1"
					logC <- log
				}
			}
		}
		wg.Done()
	}()
	go func() {
		swapped := false
		for i := 0; i < 50; i++ {
			ms := cryptoRandUint32() % 11
			if ms == 6 && !swapped {
				log := "starting to cas obj to c, id: 2, loop: " + strconv.Itoa(i)
				logC <- log
				swapped = atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&a)), unsafe.Pointer(b), unsafe.Pointer(c))
				if swapped != true {
					log = "cas result is false, id: 2"
					logC <- log
				} else {
					log = "cas result is ok, id: 2"
					logC <- log
				}
			}
		}
		wg.Done()
	}()
	go func() {
		for log := range logC {
			t.Log(log)
		}
	}()
	wg.Wait()
	time.Sleep(1 * time.Second)
}

func TestAuxIndexes(t *testing.T) {
	aux := make(xConcSklAux[uint8, *xSkipListObject], 2*xSkipListMaxLevel)
	for i := uint8(0); i < 2*xSkipListMaxLevel; i++ {
		aux[i] = &xConcSklNode[uint8, *xSkipListObject]{
			key: i,
		}
	}

	for i := uint8(0); i < xSkipListMaxLevel; i++ {
		require.Equal(t, i, aux.loadPred(int32(i)).key)
		require.Equal(t, xSkipListMaxLevel+i, aux.loadSucc(int32(i)).key)
	}

	aux.foreachPred(func(predList ...*xConcSklNode[uint8, *xSkipListObject]) {
		for i := uint8(0); i < xSkipListMaxLevel; i++ {
			require.Equal(t, i, predList[i].key)
		}
	})
	aux.foreachSucc(func(succList ...*xConcSklNode[uint8, *xSkipListObject]) {
		for i := uint8(0); i < xSkipListMaxLevel; i++ {
			require.Equal(t, xSkipListMaxLevel+i, succList[i].key)
		}
	})
}

// test remove by predecessor references:
// https://www.cs.usfca.edu/~galles/visualization/RedBlack.html
// https://github.com/minghu6/rust-minghu6/blob/master/coll_st/src/bst/rb.rs

func TestRbtreeLeftAndRightRotate(t *testing.T) {
	type checkData struct {
		color color
		val   uint64
	}

	node := &xConcSklNode[uint64, uint64]{
		vcmp: func(i, j uint64) int64 {
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
	}
	node.rbInsert(52)
	expected := []checkData{
		{black, 52},
	}
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

	node.rbInsert(47)
	expected = []checkData{
		{red, 47}, {black, 52},
	}
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

	node.rbInsert(3)
	expected = []checkData{
		{red, 3}, {black, 47}, {red, 52},
	}
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

	node.rbInsert(35)
	expected = []checkData{
		{black, 3}, {red, 35},
		{black, 47}, {black, 52},
	}
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

	node.rbInsert(24)
	expected = []checkData{
		{red, 3}, {black, 24}, {red, 35},
		{black, 47}, {black, 52},
	}
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

	// remove

	x, err := node.rbRemove(24)
	require.NoError(t, err)
	require.Equal(t, uint64(24), *x.vptr)
	expected = []checkData{
		{black, 3}, {red, 35},
		{black, 47}, {black, 52},
	}
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

	x, err = node.rbRemove(47)
	require.NoError(t, err)
	require.Equal(t, uint64(47), *x.vptr)
	expected = []checkData{
		{black, 3}, {black, 35},
		{black, 52},
	}
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

	x, err = node.rbRemove(52)
	require.NoError(t, err)
	require.Equal(t, uint64(52), *x.vptr)
	expected = []checkData{
		{red, 3}, {black, 35},
	}
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

	x, err = node.rbRemove(3)
	require.NoError(t, err)
	require.Equal(t, uint64(3), *x.vptr)
	expected = []checkData{
		{black, 35},
	}
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

	x, err = node.rbRemove(35)
	require.NoError(t, err)
	require.Equal(t, uint64(35), *x.vptr)
	require.Equal(t, int64(0), atomic.LoadInt64(&node.count))

}

func TestRandomInsertAndRemoveRbtree_SequentialNumber(t *testing.T) {
	total := uint64(100)
	insertTotal := uint64(float64(total) * 0.8)
	removeTotal := uint64(float64(total) * 0.2)

	node := &xConcSklNode[uint64, uint64]{
		vcmp: func(i, j uint64) int64 {
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
	}

	for i := uint64(0); i < insertTotal; i++ {
		node.rbInsert(i)
	}

	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, uint64(idx), val)
		return true
	})
	for i := insertTotal; i < removeTotal+insertTotal; i++ {
		node.rbInsert(i)
	}
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, uint64(idx), val)
		return true
	})

	for i := insertTotal; i < removeTotal+insertTotal; i++ {
		if i == 92 {
			x := node.rbSearch(node.root, func(x *xNode[uint64]) int64 {
				return node.vcmp(i, *x.vptr)
			})
			require.Equal(t, uint64(92), *x.vptr)
		}
		x, err := node.rbRemove(i)
		require.NoError(t, err)
		require.Equal(t, i, *x.vptr)
	}

	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, uint64(idx), val)
		return true
	})
}

func TestRandomInsertAndRemoveRbtree_ReverseSequentialNumber(t *testing.T) {
	total := int64(100)
	insertTotal := int64(float64(total) * 0.8)
	removeTotal := int64(float64(total) * 0.2)

	node := &xConcSklNode[uint64, int64]{
		vcmp: func(i, j int64) int64 {
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
	}

	for i := insertTotal - 1; i >= 0; i-- {
		node.rbInsert(i)
	}

	node.rbPreorderTraversal(func(idx int64, color color, val int64) bool {
		require.Equal(t, int64(idx), val)
		return true
	})
	for i := removeTotal + insertTotal - 1; i >= insertTotal; i-- {
		node.rbInsert(i)
	}
	node.rbPreorderTraversal(func(idx int64, color color, val int64) bool {
		require.Equal(t, int64(idx), val)
		return true
	})

	for i := insertTotal; i < removeTotal+insertTotal; i++ {
		if i == 92 {
			x := node.rbSearch(node.root, func(x *xNode[int64]) int64 {
				return node.vcmp(i, *x.vptr)
			})
			require.Equal(t, int64(92), *x.vptr)
		}
		x, err := node.rbRemove(i)
		require.NoError(t, err)
		require.Equal(t, i, *x.vptr)
	}
	node.rbPreorderTraversal(func(idx int64, color color, val int64) bool {
		require.Equal(t, int64(idx), val)
		return true
	})
}

func TestRandomInsertAndRemoveRbtree_RandomMonotonicNumber(t *testing.T) {
	total := uint64(1000000)
	insertTotal := uint64(float64(total) * 0.8)
	removeTotal := uint64(float64(total) * 0.2)

	idGen, _ := id.MonotonicNonZeroID()
	insertElements := make([]uint64, 0, insertTotal)
	removeElements := make([]uint64, 0, removeTotal)

	ignore := uint32(0)
	for {
		num := idGen.NumberUUID()
		if ignore > 0 {
			ignore--
			continue
		}
		ignore = cryptoRandUint32() % 100
		if ignore&0x1 == 0 && uint64(len(insertElements)) < insertTotal {
			insertElements = append(insertElements, num)
		} else if ignore&0x1 == 1 && uint64(len(removeElements)) < removeTotal {
			removeElements = append(removeElements, num)
		}
		if uint64(len(insertElements)) == insertTotal && uint64(len(removeElements)) == removeTotal {
			break
		}
	}

	shuffle := func(arr []uint64) {
		count := uint32(len(arr) >> 2)
		for i := uint32(0); i < count; i++ {
			j := cryptoRandUint32() % (i + 1)
			arr[i], arr[j] = arr[j], arr[i]
		}
	}

	shuffle(insertElements)
	shuffle(removeElements)

	node := &xConcSklNode[uint64, uint64]{
		vcmp: func(i, j uint64) int64 {
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
	}

	for i := uint64(0); i < insertTotal; i++ {
		node.rbInsert(insertElements[i])
	}
	sort.Slice(insertElements, func(i, j int) bool {
		return insertElements[i] < insertElements[j]
	})
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, insertElements[idx], val)
		return true
	})
	for i := uint64(0); i < removeTotal; i++ {
		node.rbInsert(removeElements[i])
	}
	for i := uint64(0); i < removeTotal; i++ {
		x, err := node.rbRemove(removeElements[i])
		require.NoError(t, err)
		require.Equalf(t, removeElements[i], *x.vptr, "value exp: %d, real: %d\n", removeElements[i], *x.vptr)
	}
	node.rbPreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, insertElements[idx], val)
		return true
	})
}
