package list

import (
	"testing"
)

func BenchmarkXArenaSklElement(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		obj := &xArenaSklElement[uint64, []byte]{}
		obj.indices = make([]*xArenaSklNode[uint64, []byte], 10)
		_ = obj
	}
}

func BenchmarkXArenaSklElement2(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		obj := new(xArenaSklElement[uint64, []byte])
		obj.indices = make([]*xArenaSklNode[uint64, []byte], 10)
		_ = obj
	}
}
