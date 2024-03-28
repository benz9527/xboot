package list

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"strconv"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestXConcSklBuffer(t *testing.T) {
	obj := xSklObject{}
	buffer := newXConcSklBuffer(100, unsafe.Sizeof(obj), unsafe.Alignof(obj))
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

	arenaCap, total := 100, 1001

	require.Panics(t, func() {
		newAutoGrowthArena[*testObj](uint32(arenaCap), 128)
	})

	arena := newAutoGrowthArena[testObj](uint32(arenaCap), 128)
	defer arena.free()

	objs := make([]*testObj, 0, total)
	for i := 0; i < total; i++ {
		obj, ok := arena.allocate()
		require.True(t, ok)
		require.NotNil(t, obj)
		x := int32(i + 100)
		buf := bytes.NewBuffer([]byte{})
		binary.Write(buf, binary.BigEndian, x)
		obj.arr = buf.Bytes()
		objs = append(objs, obj)
	}

	for i := 0; i < 10; i++ {
		runtime.GC()
		time.Sleep(2 * time.Millisecond)
	}
	i := 0
	for ; i < total; i++ {
		// If we access the recycled arr mem, occur fatal throw, unable to catch!
		// unexpected fault address 0xc000168000
		// fatal error: fault
		// [signal 0xc0000005 code=0x0 addr=0xc000168000 pc=0xd76f0e]
		// t.Logf("after gc, i: %d; arr len: %d; arr value: %v\n", i, len(objs[i].arr), objs[i].arr) // maybe exit(1)
		t.Logf("after gc, i: %d; arr len: %d\n", i, len(objs[i].arr)) // okay but abnormal actually.
	}
}

func TestAutoGrowthArena_unsafe_sliceGCRelease(t *testing.T) {
	type testObj struct {
		arr unsafe.Pointer
	}
	t.Logf("size of unsafe arr pointer: %d\n", unsafe.Sizeof(testObj{}.arr))

	type testObjWrapper struct {
		arrRef []byte  // attempt to hold arr lifecycle for testObj
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
	buffer := newXConcSklBuffer(uintptr(n), unsafe.Sizeof(node), unsafe.Alignof(node))
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
	buffer := newXConcSklBuffer(uintptr(n), unsafe.Sizeof(node), unsafe.Alignof(node))
	defer buffer.free()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = buffer.allocate()
	}
	b.StopTimer()
	b.ReportAllocs()
}
