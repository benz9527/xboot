package list

import (
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
			log := "cas load a val, obj: " + (*obj)(val).id + ", id: 1"
			runtime.Gosched()
			logC <- log
		}
		wg.Done()
	}()
	go func() {
		for i := 0; i < 200; i++ {
			val := atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&a)))
			log := "cas load a val, obj: " + (*obj)(val).id + ", id: 2"
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
		aux[i] = &xConcSkipListNode[uint8, *xSkipListObject]{
			key: i,
		}
	}

	for i := uint8(0); i < xSkipListMaxLevel; i++ {
		require.Equal(t, i, aux.loadPred(int32(i)).key)
		require.Equal(t, xSkipListMaxLevel+i, aux.loadSucc(int32(i)).key)
	}

	aux.foreachPred(func(predList ...*xConcSkipListNode[uint8, *xSkipListObject]) {
		for i := uint8(0); i < xSkipListMaxLevel; i++ {
			require.Equal(t, i, predList[i].key)
		}
	})
	aux.foreachSucc(func(succList ...*xConcSkipListNode[uint8, *xSkipListObject]) {
		for i := uint8(0); i < xSkipListMaxLevel; i++ {
			require.Equal(t, xSkipListMaxLevel+i, succList[i].key)
		}
	})
}
