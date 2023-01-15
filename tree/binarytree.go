package tree

import "golang.org/x/exp/constraints"

type BinaryTree[T constraints.Ordered] struct {
	root *Node[T]
}

func (t *BinaryTree[T]) Insert(data T) {
	n := &Node[T]{Data: data}
	tgt := t.root

	for i := tgt; i != nil; {
		tgt = i
		if data < i.Data {
			i = i.left
		} else {
			i = i.right
		}
	}

	if tgt == nil {
		t.root = n
		return
	}

	if data < tgt.Data {
		tgt.left = n
	} else {
		tgt.right = n
	}
}

func (t *BinaryTree[T]) TraverseAsc(callback func(d T)) {
	t.root.traverseAsc(callback)
}

type Node[T constraints.Ordered] struct {
	Data  T
	left  *Node[T]
	right *Node[T]
}

func (n *Node[T]) traverseAsc(callback func(d T)) {
	if n != nil {
		n.left.traverseAsc(callback)
		callback(n.Data)
		n.right.traverseAsc(callback)
	}
}
