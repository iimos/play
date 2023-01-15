package tree_test

import (
	"testing"

	"github.com/iimos/play/tree"
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

func TestBinaryTreeSort(t *testing.T) {
	var tr tree.BinaryTree[int]

	if !slices.Equal(dumpTree(tr), []int{}) {
		t.Errorf("!= []")
	}

	tr.Insert(1)

	if !slices.Equal(dumpTree(tr), []int{1}) {
		t.Errorf("!= [1]")
	}

	tr.Insert(0)
	tr.Insert(-1)

	if !slices.Equal(dumpTree(tr), []int{-1, 0, 1}) {
		t.Errorf("!= [-1,0,1]")
	}
}

func dumpTree[T constraints.Ordered](tr tree.BinaryTree[T]) (values []T) {
	tr.TraverseAsc(func(v T) {
		values = append(values, v)
	})
	return values
}
