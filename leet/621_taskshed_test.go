package leet_621

import (
	"container/heap"
	"fmt"
	"testing"
)

type task struct {
	t     byte
	count uint
}

type queue []task

func (q queue) Len() int            { return len(q) }
func (q queue) Less(i, j int) bool  { return q[i].count > q[j].count }
func (q queue) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *queue) Push(x interface{}) { *q = append(*q, x.(task)) }
func (q *queue) Pop() interface{} {
	n := len(*q)
	x := (*q)[n-1]
	*q = (*q)[:n-1]
	return x
}

func leastInterval(tasks []byte, n int) int {
	var counts ['Z' - 'A' + 1]uint
	var uniq int
	for _, t := range tasks {
		if counts[t-'A'] == 0 {
			uniq++
		}
		counts[t-'A']++
	}

	q := make(queue, 0, uniq)
	for t, cnt := range counts {
		if cnt > 0 {
			q = append(q, task{t: byte(t), count: cnt})
		}
	}
	heap.Init(&q)

	waits := make(map[int]task, len(tasks))

	steps := 0
	for q.Len() > 0 || len(waits) > 0 {
		if w, ok := waits[steps]; ok {
			delete(waits, steps)
			heap.Push(&q, w)
		}
		//fmt.Printf("%v | %v\n", q, waits)
		if q.Len() > 0 {
			head := heap.Pop(&q).(task)
			head.count--
			if head.count > 0 {
				waits[steps+n+1] = head
			}
			fmt.Print(string('A' + head.t))
		} else {
			fmt.Print("-")
		}
		steps++
	}
	fmt.Print("\n")
	return steps
}

func Test(t *testing.T) {
	leastInterval([]byte("AAAABBC"), 0)
}
