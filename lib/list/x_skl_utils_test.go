package list

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestOptimisticLock(t *testing.T) {
	lock := new(spinMutex)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		startTime := time.Now()
		lock.lock(1)
		defer wg.Done()
		defer lock.unlock(1)
		defer func() {
			t.Logf("1 elapsed: %d\n", time.Since(startTime).Milliseconds())
		}()
		ms := cryptoRandUint32() % 11
		t.Logf("id: 1, obtain spin lock, sleep ms : %d\n", ms)
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}()
	go func() {
		startTime := time.Now()
		lock.lock(1)
		defer wg.Done()
		defer lock.unlock(1)
		defer func() {
			t.Logf("2 elapsed: %d\n", time.Since(startTime).Milliseconds())
		}()
		ms := cryptoRandUint32() % 11
		t.Logf("id: 2, obtain spin lock, sleep ms : %d\n", ms)
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}()
	wg.Wait()
}

func TestFlagBitSetBitsAs(t *testing.T) {
	type testcase struct {
		name        string
		targetBits  uint32
		value       uint32
		fb          flagBits
		expectValue uint32
	}
	testcases := []testcase{
		{
			name:        "0x0000 set 0x0001 to 0x00FF as 0x0001",
			targetBits:  0x00FF,
			value:       0x0001,
			fb:          flagBits{},
			expectValue: 0x0001,
		}, {
			name:        "0x0000 set 0x00FF to 0x00FF as 0x00FF",
			targetBits:  0x00FF,
			value:       0x00FF,
			fb:          flagBits{},
			expectValue: 0x00FF,
		}, {
			name:        "0x01FF set 0x00FF to 0x00FF as 0x01FF",
			targetBits:  0x00FF,
			value:       0x00FF,
			fb:          flagBits{bits: 0x01FF},
			expectValue: 0x01FF,
		}, {
			name:        "0x01FF set 0x00FF to 0x0000 as 0x0100",
			targetBits:  0x00FF,
			value:       0x0000,
			fb:          flagBits{bits: 0x01FF},
			expectValue: 0x0100,
		}, {
			name:        "0x03FF set 0x0300 to 0x0001 as 0x01FF",
			targetBits:  0x0300,
			value:       0x0001,
			fb:          flagBits{bits: 0x03FF},
			expectValue: 0x01FF,
		}, {
			name:        "0x00FF set 0x0300 to 0x0003 as 0x03FF",
			targetBits:  0x0300,
			value:       0x0003,
			fb:          flagBits{bits: 0x00FF},
			expectValue: 0x03FF,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			tc.fb.setBitsAs(tc.targetBits, tc.value)
			require.Equal(tt, tc.expectValue, tc.fb.bits)
		})
	}
}

func TestFlagBitBitsAreEqualTo(t *testing.T) {
	type testcase struct {
		name        string
		targetBits  uint32
		value       uint32
		fb          flagBits
		expectValue bool
	}
	testcases := []testcase{
		{
			name:        "0x0001 get 0x00FF are equal to 0x0001",
			targetBits:  0x00FF,
			value:       0x0001,
			fb:          flagBits{bits: 0x0001},
			expectValue: true,
		}, {
			name:        "0x0301 get 0x0700 are equal to 0x0003",
			targetBits:  0x0700,
			value:       0x0003,
			fb:          flagBits{bits: 0x0301},
			expectValue: true,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			res := tc.fb.areEqual(tc.targetBits, tc.value)
			require.True(tt, res)
		})
	}
}

func TestArenaBuffer(t *testing.T) {
	obj := xSklObject{}
	buffer := newArenaBuffer(100, unsafe.Sizeof(obj), unsafe.Alignof(obj))
	optr, ok := buffer.allocate()
	require.True(t, ok)
	(*xSklObject)(optr).id = "abc"
	o := *(*xSklObject)(optr)
	t.Log(o)
	op := &o
	op2 := (*xSklObject)(optr)
	t.Logf("%p, %p, %p\n", op, op2, (*xSklObject)(optr))

	optr2, ok := buffer.allocate()
	require.True(t, ok)
	(*xSklObject)(optr2).id = "bcd"
	o2 := *(*xSklObject)(optr2)
	t.Log(o2)
	op_2 := &o2
	op_2_1 := (*xSklObject)(optr2)
	t.Logf("%p, %p, %p\n", op_2, op_2_1, (*xSklObject)(optr2))
}

func TestAutoGrowthArena_xConcSklNode(t *testing.T) {
	arenaCap, total := 10, 101

	require.Panics(t, func() {
		newAutoGrowthArena[*xConcSklNode[uint64, []string]](uint32(arenaCap), 128)
	})

	arena := newAutoGrowthArena[xConcSklNode[uint64, []string]](uint32(arenaCap), 128)
	defer arena.free()
	rand := cryptoRandUint32() % uint32(total)
	for i := 0; i < total; i++ {
		obj, ok := arena.allocate()
		require.True(t, ok)
		require.NotNil(t, obj)
		if i == int(rand) {
			arena.recycle(obj)
		}
	}
	require.Equal(t, 10, arena.bufLen())
	require.Equal(t, 0, arena.recLen())
	require.Equal(t, uint64(100), arena.objLen())
}

func TestAutoGrowthArena_sliceGCRelease(t *testing.T) {
	type testObj struct {
		arr []byte
	}

	// If we access the recycled arr mem, occur fatal throw, unable to catch!
	// Without the holder, here must be panic.
	// unexpected fault address 0xc000168000
	// fatal error: fault
	// [signal 0xc0000005 code=0x0 addr=0xc000168000 pc=0xd76f0e]
	type testObjWrapper struct {
		arrRef []byte // attempt to hold arr lifecycle for testObj
		obj    *testObj
	}

	arenaCap, total := 100, 1001

	require.Panics(t, func() {
		newAutoGrowthArena[*testObj](uint32(arenaCap), 128)
	})

	arena := newAutoGrowthArena[testObj](uint32(arenaCap), 128)
	defer arena.free()

	objs := make([]*testObjWrapper, 0, total)
	for i := 0; i < total; i++ {
		obj, ok := arena.allocate()
		require.True(t, ok)
		require.NotNil(t, obj)
		x := int32(i + 100)
		buf := bytes.NewBuffer([]byte{})
		binary.Write(buf, binary.BigEndian, x)

		w := &testObjWrapper{
			arrRef: buf.Bytes(),
		}
		obj.arr = w.arrRef
		w.obj = obj
		objs = append(objs, w)
	}

	for i := 0; i < 10; i++ {
		runtime.GC()
		time.Sleep(2 * time.Millisecond)
	}
	i := 0
	for ; i < total; i++ {
		t.Logf("after gc, i: %d; arr len: %d; arr value: %v\n", i, len(objs[i].obj.arr), objs[i].obj.arr) // maybe exit(1)
	}
}

func TestAutoGrowthArena_unsafe_sliceGCRelease(t *testing.T) {
	type testObj struct {
		arr unsafe.Pointer
	}
	t.Logf("size of unsafe arr pointer: %d\n", unsafe.Sizeof(testObj{}.arr))

	type testObjWrapper struct {
		arrRef []byte // attempt to hold arr lifecycle for testObj
		obj    *testObj
	}

	arenaCap, total := 100, 1001

	require.Panics(t, func() {
		newAutoGrowthArena[*testObj](uint32(arenaCap), 128)
	})

	arena := newAutoGrowthArena[testObj](uint32(arenaCap), 128)
	defer arena.free()

	objs := make([]*testObjWrapper, 0, total)
	for i := 0; i < total; i++ {
		obj, ok := arena.allocate()
		require.True(t, ok)
		require.NotNil(t, obj)
		x := i + 100
		b := bytes.NewBufferString(strconv.Itoa(x)).Bytes()
		t.Log(b)
		w := &testObjWrapper{
			arrRef: b,
		}
		obj.arr = unsafe.Pointer(unsafe.SliceData(w.arrRef))
		w.obj = obj
		objs = append(objs, w)
	}

	for i := 0; i < 10; i++ {
		runtime.GC()
		time.Sleep(2 * time.Millisecond)
	}
	i := 0
	for ; i < total; i++ {
		arr := unsafe.Slice((*byte)(objs[i].obj.arr), 4)
		t.Logf("after gc, i: %d; arr len: %d; arr value: %v\n", i, len(arr), arr)
	}
}

func BenchmarkXConcSklBuffer_xConcSklNode(b *testing.B) {
	n := b.N
	node := xConcSklNode[uint64, []byte]{}
	buffer := newArenaBuffer(uintptr(n), unsafe.Sizeof(node), unsafe.Alignof(node))
	defer buffer.free()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = buffer.allocate()
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkXConcSklBuffer_xNode(b *testing.B) {
	n := b.N
	node := xNode[[]byte]{}
	buffer := newArenaBuffer(uintptr(n), unsafe.Sizeof(node), unsafe.Alignof(node))
	defer buffer.free()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = buffer.allocate()
	}
	b.StopTimer()
	b.ReportAllocs()
}
