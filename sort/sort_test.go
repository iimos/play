package sort

import (
	"fmt"
	"math"
	"testing"

	"golang.org/x/exp/constraints"
	"golang.org/x/exp/rand"
	"golang.org/x/exp/slices"
)

func sizeOf[T constraints.Integer]() uint {
	x := uint16(1 << 8)
	y := uint32(2 << 16)
	z := uint64(4 << 32)
	return 1 + uint(T(x))>>8 + uint(T(y))>>16 + uint(T(z))>>32
}

func maxOfType[T constraints.Integer]() T {
	ones := ^T(0)
	if ones < 0 {
		return ones ^ (ones << (8*sizeOf[T]() - 1))
	}
	return ones
}

func randomInts[T constraints.Integer](n int) []T {
	ints := make([]T, n)
	for i := 0; i < n; i++ {
		ints[i] = T(rand.Intn(int(maxOfType[T]())))
	}
	return ints
}

func orderedInts[T constraints.Integer](n int) []T {
	ints := make([]T, n)
	for i := 0; i < n; i++ {
		ints[i] = T(i)
	}
	return ints
}

func lowcardInts[T constraints.Integer](n int) []T {
	ints := make([]T, 0, n)
	for i := 0; i <= n/10; i++ {
		x := T(rand.Intn(int(maxOfType[T]())))
		for j := 0; j < 10; j++ {
			ints = append(ints, x)
		}
	}
	rand.Shuffle(n, func(i, j int) {
		ints[i], ints[j] = ints[j], ints[i]
	})
	return ints[:n]
}

type testCase[T constraints.Integer] struct {
	name           string
	sortFunc       func(a *[]T)
	arrayGenerator func(n int) []T
}

func stdPDQSort[T constraints.Ordered](arr *[]T) {
	slices.Sort(*arr)
}

func genTestCases[T constraints.Integer]() []testCase[T] {
	var sortfuncs = []struct {
		name string
		fn   func(a *[]T)
	}{
		{"QSort", QSort[T]},
		{"InsertionSort", InsertionSort[T]},
		{"MergeSort", MergeSort[T]},
		{"HeapSort", HeapSort[T]},
		{"BinaryTreeSort", BinaryTreeSort[T]},
		{"std PDQSort", stdPDQSort[T]},
		// {"GravitySort", GravitySort[T]}, // it's too slow
	}

	var arrgens = []struct {
		name   string
		arrgen func(n int) []T
	}{
		{"random", randomInts[T]},
		{"ordered", orderedInts[T]},
		{"low card", lowcardInts[T]},
	}

	cases := make([]testCase[T], 0)

	for _, ag := range arrgens {
		for _, f := range sortfuncs {
			cases = append(cases, testCase[T]{
				name:           f.name + "_" + ag.name,
				sortFunc:       f.fn,
				arrayGenerator: ag.arrgen,
			})
		}
	}

	return cases
}

func TestSort(t *testing.T) {
	for n := range []int{0, 1, 2, 10, 100} {
		for _, tc := range genTestCases[int32]() {
			name := fmt.Sprintf("%s_n%d", tc.name, n)
			t.Run(name, func(tt *testing.T) {
				a := tc.arrayGenerator(n)
				b := slices.Clone(a)

				tc.sortFunc(&a)
				slices.Sort(b)

				if !slices.Equal(a, b) {
					tt.Logf("%v\n", a)
					tt.Logf("%v\n", b)
					tt.Fail()
				}
			})
		}
	}
}

func BenchmarkSort(b *testing.B) {
	for _, tc := range genTestCases[int32]() {
		b.Run(tc.name, func(b *testing.B) {
			arr := tc.arrayGenerator(100_000)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				a := slices.Clone(arr)
				tc.sortFunc(&a)
			}
		})
	}
}

func TestMaxOf(t *testing.T) {
	if math.MaxInt8 != maxOfType[int8]() {
		t.Errorf("maxOfType wrong for int8")
	}
	if math.MaxInt16 != maxOfType[int16]() {
		t.Errorf("maxOfType wrong for int16")
	}
	if math.MaxInt32 != maxOfType[int32]() {
		t.Errorf("maxOfType wrong for int32")
	}
	if math.MaxInt64 != maxOfType[int64]() {
		t.Errorf("maxOfType wrong for int64")
	}
	if math.MaxInt != maxOfType[int]() {
		t.Errorf("maxOfType wrong for int")
	}

	if math.MaxUint8 != maxOfType[uint8]() {
		t.Errorf("maxOfType wrong for uint8")
	}
	if math.MaxUint16 != maxOfType[uint16]() {
		t.Errorf("maxOfType wrong for uint16")
	}
	if math.MaxUint32 != maxOfType[uint32]() {
		t.Errorf("maxOfType wrong for uint32")
	}
	if math.MaxUint64 != maxOfType[uint64]() {
		t.Errorf("maxOfType wrong for uint64")
	}
	if math.MaxUint != maxOfType[uint]() {
		t.Errorf("maxOfType wrong for uint")
	}
}
