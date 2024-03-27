package list

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func TestXConcSklBuffer(t *testing.T) {
	obj := xSklObject{}
	buffer := newXConcSklBuffer(100, unsafe.Sizeof(obj), unsafe.Alignof(obj))
	optr, ok := buffer.alloc()
	require.True(t, ok)
	(*xSklObject)(optr).id = "abc"
	o := *(*xSklObject)(optr)
	t.Log(o)
	op := &o
	op2 := (*xSklObject)(optr)
	t.Logf("%p, %p, %p\n", op, op2, (*xSklObject)(optr))

	optr2, ok := buffer.alloc()
	require.True(t, ok)
	(*xSklObject)(optr2).id = "bcd"
	o2 := *(*xSklObject)(optr2)
	t.Log(o2)
	op_2 := &o2
	op_2_1 := (*xSklObject)(optr2)
	t.Logf("%p, %p, %p\n", op_2, op_2_1, (*xSklObject)(optr2))
}

func BenchmarkXConcSklBuffer_xConcSklNode(b *testing.B) {
	n := b.N
	node := xConcSklNode[uint64, []byte]{}
	buffer := newXConcSklBuffer(uintptr(n), unsafe.Sizeof(node), unsafe.Alignof(node))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = buffer.alloc()
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkXConcSklBuffer_xNode(b *testing.B) {
	n := b.N
	node := xNode[[]byte]{}
	buffer := newXConcSklBuffer(uintptr(n), unsafe.Sizeof(node), unsafe.Alignof(node))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = buffer.alloc()
	}
	b.StopTimer()
	b.ReportAllocs()
}