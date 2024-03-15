package list

import (
	"github.com/benz9527/xboot/lib/id"
	"github.com/stretchr/testify/assert"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
)

type xNode[W SkipListWeight, O HashObject] struct {
	weight *atomic.Pointer[W]
	object *atomic.Pointer[O]
	next   *atomic.Pointer[xNode[W, O]]
}

func TestAtomicPointerCAS(t *testing.T) {
	a := &xNode[uint8, *xSkipListObject]{
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
	c := &xNode[uint8, *xSkipListObject]{
		weight: &atomic.Pointer[uint8]{},
		object: &atomic.Pointer[*xSkipListObject]{},
		next:   nil,
	}
	c.weight.Store(&w2)
	c.object.Store(&o2)

	ptr := atomic.Pointer[xNode[uint8, *xSkipListObject]]{}
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
	aux := make(xConcSkipListAuxiliary[uint8, *xSkipListObject], 2*xSkipListMaxLevel)
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

func TestRbtree(t *testing.T) {
	type checkData struct {
		color color
		val   int32
	}
	node := &xConcSklNode[int32, int32]{
		vcmp: func(i, j int32) int64 {
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
		nilLeafNode: &xNode[int32]{
			color: black,
		},
	}
	node.root = node.nilLeafNode
	for i := int32(0); i < 10; i++ {
		node.rbtreeInsertIfNotPresent(i)
	}
	expected := []checkData{
		{false, 0}, {false, 1}, {false, 2}, {false, 3},
		{false, 4}, {false, 5}, {false, 6}, {true, 7},
		{false, 8}, {true, 9},
	}
	node.rbtreePreorderTraversal(func(idx int64, color color, val int32) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})
	t.Log("delete 5")
	_, err := node.rbtreeRemoveByPred(5)
	assert.NoError(t, err)
	expected = []checkData{
		{false, 0}, {false, 1}, {false, 2}, {false, 3},
		{false, 4}, {true, 6}, {false, 7},
		{false, 8}, {true, 9},
	}
	node.rbtreePreorderTraversal(func(idx int64, color color, val int32) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

	t.Log("delete 1")
	_, err = node.rbtreeRemoveByPred(1)
	assert.NoError(t, err)
	expected = []checkData{
		{false, 0}, {true, 2}, {false, 3},
		{false, 4}, {true, 6}, {true, 7},
		{false, 8}, {true, 9},
	}
	node.rbtreePreorderTraversal(func(idx int64, color color, val int32) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

	t.Log("delete 7")
	_, err = node.rbtreeRemoveByPred(7)
	assert.NoError(t, err)
	expected = []checkData{
		{false, 0}, {true, 2}, {false, 3},
		{false, 4}, {true, 6},
		{false, 8}, {true, 9},
	}
	node.rbtreePreorderTraversal(func(idx int64, color color, val int32) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].val, val)
		return true
	})

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
		nilLeafNode: &xNode[uint64]{
			color: black,
		},
	}
	node.root = node.nilLeafNode

	for i := uint64(0); i < insertTotal; i++ {
		node.rbtreeInsertIfNotPresent(i)
	}

	t.Log("insert okay1")
	node.rbtreePreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, uint64(idx), val)
		return true
	})
	for i := insertTotal; i < removeTotal+insertTotal; i++ {
		node.rbtreeInsertIfNotPresent(i)
	}
	t.Log("insert okay2")
	node.rbtreePreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, uint64(idx), val)
		return true
	})

	for i := insertTotal; i < removeTotal+insertTotal; i++ {
		if i == 92 {
			node.rbtreePreorderTraversal(func(idx int64, color color, val uint64) bool {
				t.Logf("idx: %d, expected: %d, actual: %d\n", idx, idx, val)
				return true
			})
			vn := node.rbtreeSearch(node.root, func(vn *xNode[uint64]) int64 {
				v := *vn.val
				if v == i {
					return 0
				} else if i > v {
					return 1
				}
				return -1
			})
			require.Equal(t, uint64(92), *vn.vptr)
		}
		vn, err := node.rbtreeRemoveByPred(i)
		t.Logf("rm target: %d, rm actual: %v, err? %v\n", i, vn, err)
	}
	t.Log("remove okay")

	node.rbtreePreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, uint64(idx), val)
		return true
	})
}

func TestRandomInsertAndRemoveRbtree(t *testing.T) {
	total := uint64(100)
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
	t.Log("gen okay")
	node := &xConcSklNode[uint64, uint64]{
		vcmp: func(i, j uint64) int64 {
			if i == j {
				return 0
			} else if i > j {
				return 1
			}
			return -1
		},
		nilLeafNode: &xNode[uint64]{
			color: black,
		},
	}
	node.root = node.nilLeafNode

	for i := uint64(0); i < insertTotal; i++ {
		node.rbtreeInsertIfNotPresent(insertElements[i])
	}
	t.Log("insert okay1")
	node.rbtreePreorderTraversal(func(idx int64, color color, val uint64) bool {
		require.Equal(t, insertElements[idx], val)
		return true
	})
	for i := uint64(0); i < removeTotal; i++ {
		node.rbtreeInsertIfNotPresent(removeElements[i])
	}
	t.Log("insert okay2")
	for i := uint64(0); i < removeTotal; i++ {
		vn, err := node.rbtreeRemoveByPred(removeElements[i])
		t.Logf("rm target: %d, rm actual: %v, err? %v\n", removeElements[i], vn, err)
	}
	t.Log("remove okay")
	node.rbtreePreorderTraversal(func(idx int64, color color, val uint64) bool {
		t.Logf("idx: %d, expected: %d, actual: %d\n", idx, insertElements[idx], val)
		require.Equal(t, insertElements[idx], val)
		return true
	})
}
