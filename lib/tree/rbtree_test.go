package tree

import (
	randv2 "math/rand/v2"
	"sort"
	"testing"

	"github.com/benz9527/xboot/lib/id"
	"github.com/stretchr/testify/require"
)

func TestNilNode(t *testing.T) {
	var nilNode RBNode[uint64, uint64] = nil
	require.True(t, nilNode == nil)

	var nilNode2 *rbNode[uint64, uint64] = nil
	nilNode = nilNode2
	require.True(t, nilNode != nil)
	require.Nil(t, nilNode)
}

func TestRbtreeLeftAndRightRotate_Pred(t *testing.T) {
	type checkData struct {
		color RBColor
		key   uint64
	}

	tree := &rbTree[uint64, uint64]{
		isDesc:         false,
		isRmBorrowSucc: false,
	}

	tree.Insert(52, 1)
	expected := []checkData{
		{Black, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate[uint64, uint64](tree))
	require.NoError(t, BlackViolationValidate[uint64, uint64](tree))

	tree.Insert(47, 1)
	expected = []checkData{
		{Red, 47}, {Black, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate[uint64, uint64](tree))

	tree.Insert(3, 1)
	expected = []checkData{
		{Red, 3}, {Black, 47}, {Red, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate[uint64, uint64](tree))
	require.NoError(t, BlackViolationValidate[uint64, uint64](tree))

	tree.Insert(35, 1)
	expected = []checkData{
		{Black, 3},
		{Red, 35},
		{Black, 47},
		{Black, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate[uint64, uint64](tree))
	require.NoError(t, BlackViolationValidate[uint64, uint64](tree))

	tree.Insert(24, 1)
	expected = []checkData{
		{Red, 3},
		{Black, 24},
		{Red, 35},
		{Black, 47},
		{Black, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate[uint64, uint64](tree))
	require.NoError(t, BlackViolationValidate[uint64, uint64](tree))

	// remove

	x, err := tree.Remove(24)
	require.NoError(t, err)
	require.Equal(t, uint64(24), x.Key())
	expected = []checkData{
		{Black, 3},
		{Red, 35},
		{Black, 47},
		{Black, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate[uint64, uint64](tree))
	require.NoError(t, BlackViolationValidate[uint64, uint64](tree))

	x, err = tree.Remove(47)
	require.NoError(t, err)
	require.Equal(t, uint64(47), x.Key())
	expected = []checkData{
		{Black, 3},
		{Black, 35},
		{Black, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate[uint64, uint64](tree))
	require.NoError(t, BlackViolationValidate[uint64, uint64](tree))

	x, err = tree.Remove(52)
	require.NoError(t, err)
	require.Equal(t, uint64(52), x.Key())
	expected = []checkData{
		{Red, 3}, {Black, 35},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate[uint64, uint64](tree))
	require.NoError(t, BlackViolationValidate[uint64, uint64](tree))

	x, err = tree.Remove(3)
	require.NoError(t, err)
	require.Equal(t, uint64(3), x.Key())
	expected = []checkData{
		{Black, 35},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate[uint64, uint64](tree))
	require.NoError(t, BlackViolationValidate[uint64, uint64](tree))

	x, err = tree.Remove(35)
	require.NoError(t, err)
	require.Equal(t, uint64(35), x.Key())
	require.Equal(t, int64(0), tree.Len())
}

func TestRbtree_RemoveMin(t *testing.T) {
	type checkData struct {
		color RBColor
		key   uint64
	}

	tree := &rbTree[uint64, uint64]{
		isDesc:         false,
		isRmBorrowSucc: false,
	}

	tree.Insert(52, 1)
	tree.Insert(47, 1)
	tree.Insert(3, 1)
	tree.Insert(35, 1)
	tree.Insert(24, 1)
	expected := []checkData{
		{Red, 3},
		{Black, 24},
		{Red, 35},
		{Black, 47},
		{Black, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})

	// remove min

	x, err := tree.RemoveMin()
	require.NoError(t, err)
	require.Equal(t, uint64(3), x.Key())
	expected = []checkData{
		{Black, 24},
		{Red, 35},
		{Black, 47},
		{Black, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate(tree))
	require.NoError(t, BlackViolationValidate(tree))

	x, err = tree.RemoveMin()
	require.NoError(t, err)
	require.Equal(t, uint64(24), x.Key())
	expected = []checkData{
		{Black, 35},
		{Black, 47},
		{Black, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate(tree))
	require.NoError(t, BlackViolationValidate(tree))

	x, err = tree.RemoveMin()
	require.NoError(t, err)
	require.Equal(t, uint64(35), x.Key())
	expected = []checkData{
		{Black, 47}, {Red, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate(tree))
	require.NoError(t, BlackViolationValidate(tree))

	x, err = tree.RemoveMin()
	require.NoError(t, err)
	require.Equal(t, uint64(47), x.Key())
	expected = []checkData{
		{Black, 52},
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, expected[idx].color, color)
		require.Equal(t, expected[idx].key, key)
		return true
	})
	require.NoError(t, RedViolationValidate(tree))
	require.NoError(t, BlackViolationValidate(tree))

	x, err = tree.RemoveMin()
	require.NoError(t, err)
	require.Equal(t, uint64(52), x.Key())
	require.Equal(t, int64(0), tree.Len())
}

func rbtreeRandomInsertAndRemoveSequentialNumberRunCore(t *testing.T, rbRmBySucc bool) {
	total := uint64(1000)
	insertTotal := uint64(float64(total) * 0.8)
	removeTotal := uint64(float64(total) * 0.2)

	tree := &rbTree[uint64, uint64]{
		isDesc:         false,
		isRmBorrowSucc: rbRmBySucc,
	}

	for i := uint64(0); i < insertTotal; i++ {
		tree.Insert(i, 1)
		require.NoError(t, RedViolationValidate(tree))
		require.NoError(t, BlackViolationValidate(tree))
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, uint64(idx), key)
		return true
	})

	for i := insertTotal; i < removeTotal+insertTotal; i++ {
		tree.Insert(i, 1)
		require.NoError(t, RedViolationValidate(tree))
		require.NoError(t, BlackViolationValidate(tree))
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, uint64(idx), key)
		return true
	})

	for i := insertTotal; i < removeTotal+insertTotal; i++ {
		if i == 92 {
			x := tree.Search(tree.root, func(node RBNode[uint64, uint64]) int64 {
				if i == node.Key() {
					return 0
				} else if i < node.Key() {
					return -1
				}
				return 1
			})
			require.Equal(t, uint64(92), x.Key())
		}
		x, err := tree.Remove(i)
		require.NoError(t, err)
		require.Equal(t, i, x.Key())
		require.NoError(t, RedViolationValidate(tree))
		require.NoError(t, BlackViolationValidate(tree))
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, uint64(idx), key)
		return true
	})
}

func TestRbtreeRandomInsertAndRemove_SequentialNumber(t *testing.T) {
	type testcase struct {
		name       string
		rbRmBySucc bool
	}
	testcases := []testcase{
		{
			name: "rm by pred",
		},
		{
			name:       "rm by succ",
			rbRmBySucc: true,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			rbtreeRandomInsertAndRemoveSequentialNumberRunCore(tt, tc.rbRmBySucc)
		})
	}
}

func TestRBTreeRandomInsertAndRemove_SequentialNumber_Release(t *testing.T) {
	insertTotal := uint64(100_000)

	tree := &rbTree[uint64, uint64]{
		isDesc:         false,
		isRmBorrowSucc: false,
	}

	rand := uint64(randv2.Uint32() % 1_000)
	for i := uint64(0); i < insertTotal; i++ {
		tree.Insert(i, 1)
		if i%1000 == rand {
			require.NoError(t, RedViolationValidate(tree))
			require.NoError(t, BlackViolationValidate(tree))
		}
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, uint64(idx), key)
		return true
	})
	tree.Release()
	require.Equal(t, int64(0), tree.Len())
	require.Nil(t, tree.Root())
}

func TestRbtreeRandomInsertAndRemove_ReverseSequentialNumber(t *testing.T) {
	total := int64(10000)
	insertTotal := int64(float64(total) * 0.8)
	removeTotal := int64(float64(total) * 0.2)

	tree := &rbTree[int64, uint64]{
		isDesc:         true,
		isRmBorrowSucc: false,
	}

	rand := int64(randv2.Uint32() % 1_000)
	for i := insertTotal - 1; i >= 0; i-- {
		tree.Insert(i, 1)
		if i%1000 == rand {
			require.NoError(t, RedViolationValidate(tree))
			require.NoError(t, BlackViolationValidate(tree))
		}
	}
	tree.Foreach(func(idx int64, color RBColor, key int64, val uint64) bool {
		require.Equal(t, int64(insertTotal-1-idx), key)
		return true
	})

	for i := removeTotal + insertTotal - 1; i >= insertTotal; i-- {
		tree.Insert(i, 1)
	}
	tree.Foreach(func(idx int64, color RBColor, key int64, val uint64) bool {
		require.Equal(t, int64(removeTotal+insertTotal-1-idx), key)
		return true
	})

	for i := insertTotal; i < removeTotal+insertTotal; i++ {
		if i == 92 {
			x := tree.Search(tree.root, func(x RBNode[int64, uint64]) int64 {
				if i == x.Key() {
					return 0
				} else if i < x.Key() {
					return 1
				}
				return -1
			})
			require.Equal(t, int64(92), x.Key())
		}
		x, err := tree.Remove(i)
		require.NoError(t, err)
		require.Equal(t, i, x.Key())
	}
	tree.Foreach(func(idx int64, color RBColor, key int64, val uint64) bool {
		require.Equal(t, int64(insertTotal-1-idx), key)
		return true
	})
}

func rbtreeRandomInsertAndRemove_RandomMonoNumberRunCore(t *testing.T, total uint64, rbRmBySucc bool, violationCheck bool) {
	insertTotal := uint64(float64(total) * 0.8)
	removeTotal := uint64(float64(total) * 0.2)

	idGen, _ := id.MonotonicNonZeroID()
	insertElements := make([]uint64, 0, insertTotal)
	removeElements := make([]uint64, 0, removeTotal)

	ignore := uint32(0)

	for {
		num := idGen.Number()
		if ignore > 0 {
			ignore--
			continue
		}
		ignore = randv2.Uint32() % 100
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
			j := randv2.Uint32() % (i + 1)
			arr[i], arr[j] = arr[j], arr[i]
		}
	}

	shuffle(insertElements)
	shuffle(removeElements)

	tree := &rbTree[uint64, uint64]{
		isDesc:         false,
		isRmBorrowSucc: rbRmBySucc,
	}

	for i := uint64(0); i < insertTotal; i++ {
		tree.Insert(insertElements[i], i)
		if violationCheck {
			require.NoError(t, RedViolationValidate(tree))
			require.NoError(t, BlackViolationValidate(tree))
		}
	}
	sort.Slice(insertElements, func(i, j int) bool {
		return insertElements[i] < insertElements[j]
	})
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, insertElements[idx], key)
		return true
	})

	for i := uint64(0); i < removeTotal; i++ {
		tree.Insert(removeElements[i], 1)
		if violationCheck {
			require.NoError(t, RedViolationValidate(tree))
			require.NoError(t, BlackViolationValidate(tree))
		}
	}
	require.NoError(t, RedViolationValidate(tree))
	require.NoError(t, BlackViolationValidate(tree))

	for i := uint64(0); i < removeTotal; i++ {
		x, err := tree.Remove(removeElements[i])
		require.NoError(t, err)
		require.Equalf(t, removeElements[i], x.Key(), "value exp: %d, real: %d\n", removeElements[i], x.Key())
		if violationCheck {
			require.NoError(t, RedViolationValidate(tree))
			require.NoError(t, BlackViolationValidate(tree))
		}
	}
	tree.Foreach(func(idx int64, color RBColor, key uint64, val uint64) bool {
		require.Equal(t, insertElements[idx], key)
		return true
	})
}

func TestRbtreeRandomInsertAndRemove_RandomMonotonicNumber(t *testing.T) {
	type testcase struct {
		name           string
		rbRmBySucc     bool
		total          uint64
		violationCheck bool
	}
	testcases := []testcase{
		{
			name:  "rm by pred 1000000",
			total: 1000000,
		},
		{
			name:       "rm by succ 1000000",
			rbRmBySucc: true,
			total:      1000000,
		},
		{
			name:           "violation check rm by pred 10000",
			total:          10000,
			violationCheck: true,
		},
		{
			name:           "violation check rm by succ 10000",
			rbRmBySucc:     true,
			total:          10000,
			violationCheck: true,
		},
		{
			name:           "violation check rm by pred 20000",
			total:          20000,
			violationCheck: true,
		},
		{
			name:           "violation check rm by succ 20000",
			rbRmBySucc:     true,
			total:          20000,
			violationCheck: true,
		},
	}
	t.Parallel()
	for _, tc := range testcases {
		t.Run(tc.name, func(tt *testing.T) {
			rbtreeRandomInsertAndRemove_RandomMonoNumberRunCore(tt, tc.total, tc.rbRmBySucc, tc.violationCheck)
		})
	}
}

func BenchmarkRBTree_Random(b *testing.B) {
	testByBytes := []byte(`abc`)

	b.StopTimer()
	tree := NewRBTree[int, []byte]()

	rngArr := make([]int, 0, b.N)
	for i := 0; i < b.N; i++ {
		rngArr = append(rngArr, randv2.Int())
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		err := tree.Insert(rngArr[i], testByBytes)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkRBTree_Serial(b *testing.B) {
	testByBytes := []byte(`abc`)

	b.StopTimer()
	tree := NewRBTree[int, []byte]()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tree.Insert(i, testByBytes)
	}
}
