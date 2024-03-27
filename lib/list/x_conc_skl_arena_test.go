package list

import (
	"testing"
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
