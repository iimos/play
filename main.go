package main

import (
	"fmt"

	"github.com/iimos/play/sort"
)

func main() {
	arr := []uint64{7, 6, 99, 1, 9, 10}
	fmt.Printf("original: %v\n", arr)
	sort.HeapSort(&arr)
	fmt.Printf("  sorted: %v\n", arr)
}
