package sort

import (
	"math/rand"

	"github.com/iimos/play/tree"
	"golang.org/x/exp/constraints"
)

func QSort[T constraints.Ordered](arr *[]T) {
	a := *arr

	if len(a) <= 2 {
		sort2(arr)
		return
	}

	if len(a) <= 16 {
		InsertionSort(arr)
		return
	}

	pivot := rand.Intn(len(a))
	a[0], a[pivot] = a[pivot], a[0]

	k := 0
	for i := 0; i < len(a); i++ {
		if a[i] < a[0] {
			k++
			a[i], a[k] = a[k], a[i]
		}
	}
	a[0], a[k] = a[k], a[0]

	s1 := a[:k]
	QSort(&s1)

	s2 := a[k+1:]
	QSort(&s2)
}

func InsertionSort[T constraints.Ordered](arr *[]T) {
	a := *arr

	if len(a) <= 2 {
		sort2(arr)
		return
	}

	if len(a) <= 16 {
		// it's faster on small arrays which are fits into cpu cache line
		for i := 1; i < len(a); i++ {
			for j := i; j > 0 && a[j] < a[j-1]; j-- {
				a[j], a[j-1] = a[j-1], a[j]
			}
		}
	} else {
		for i := 1; i < len(a); i++ {
			j, ok := binarySearch(i-1, func(j int) bool {
				return a[j] > a[i]
			})
			if ok && j < i {
				t := a[i]
				copy(a[j+1:], a[j:i])
				a[j] = t
			}
		}
	}
}

func binarySearch(n int, cond func(i int) bool) (idx int, ok bool) {
	i, j := 0, n
	for i < j {
		m := int(uint(i+j) >> 1)
		if cond(m) {
			j = m
		} else {
			i = m + 1
		}
	}

	if i > n || !cond(i) {
		return 0, false
	}
	return i, true
}

func MergeSort[T constraints.Ordered](arr *[]T) {
	if len(*arr) <= 2 {
		sort2(arr)
		return
	}
	ret := make([]T, len(*arr))
	mergeSort(*arr, &ret, 0)
	*arr = ret
}

func mergeSort[T constraints.Ordered](arr []T, ret *[]T, p int) {
	if len(arr) <= 2 {
		sort2(&arr)
		for _, v := range arr {
			(*ret)[p] = v
			p++
		}
		return
	}
	m := len(arr) / 2
	mergeSort(arr[:m], ret, p)
	mergeSort(arr[m:], ret, p+m)
	merge(arr[:m], arr[m:], ret, p)
}

func merge[T constraints.Ordered](a, b []T, ret *[]T, p int) {
	i, j := 0, 0

	for i < len(a) && j < len(b) {
		if a[i] < b[j] {
			(*ret)[p] = a[i]
			i++
		} else {
			(*ret)[p] = b[j]
			j++
		}
		p++
	}
	for i < len(a) {
		(*ret)[p] = a[i]
		i++
		p++
	}
	for j < len(b) {
		(*ret)[p] = b[j]
		j++
		p++
	}
}

func HeapSort[T constraints.Ordered](arr *[]T) {
	a := *arr

	for i := len(a) / 2; i >= 0; i-- {
		heapify(a, i)
	}

	for i := len(a) - 1; i >= 0; i-- {
		a[0], a[i] = a[i], a[0]
		heapify(a[:i], 0)
	}
}

func heapify[T constraints.Ordered](arr []T, root int) {
	for {
		c1 := 2*root + 1
		if c1 >= len(arr) || c1 < 0 {
			return
		}

		cmax := c1
		c2 := c1 + 1
		if c2 < len(arr) && c2 > 0 && arr[cmax] < arr[c2] {
			cmax = c2
		}

		if arr[root] >= arr[cmax] {
			return
		}

		arr[root], arr[cmax] = arr[cmax], arr[root]
		root = cmax
	}
}

func BinaryTreeSort[T constraints.Ordered](arr *[]T) {
	t := &tree.BinaryTree[T]{}
	for _, v := range *arr {
		t.Insert(v)
	}

	i := 0
	t.TraverseAsc(func(d T) {
		(*arr)[i] = d
		i++
	})
}

// https://en.wikipedia.org/wiki/Bead_sort
func GravitySort[T constraints.Integer](arr *[]T) {
	a := *arr

	if len(a) <= 2 {
		sort2(arr)
		return
	}

	min, max := a[0], a[0]

	for i := 1; i < len(a); i++ {
		if max < a[i] {
			max = a[i]
		}
		if a[i] < min {
			min = a[i]
		}
	}

	min -= 1 // to eliminate zero radix

	radixes := make([]uint8, max-min)
	for k, v := range a {
		v -= min
		for i := 0; i < int(v); i++ {
			radixes[i]++
		}
		a[k] = 0
	}

	p := 0
	for i := len(radixes) - 1; i >= 0; i-- {
		for j := p; j < int(radixes[i]); j++ {
			a[len(a)-1-p] = T(i+1) + min
			p++
		}
	}
}

func sort2[T constraints.Ordered](arr *[]T) {
	a := *arr
	if len(a) < 2 {
		return
	}
	if len(a) == 2 {
		if a[1] < a[0] {
			a[0], a[1] = a[1], a[0]
		}
		return
	}
	panic("slice too big")
}
