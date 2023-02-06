package main

import (
	"fmt"
	"sync/atomic"
	"time"

	"golang.design/x/thread"
)

func testAtomic() {
	var sum1 int64 = 0
	var sum2 int64 = 0

	counter := func(tid uint64, useAtomic bool) {
		fmt.Printf("thread %d: start\n", tid)

		if useAtomic {
			for i := 0; i < 10_000_000; i++ {
				atomic.AddInt64(&sum1, 1)
				sum2 += 1
			}
		} else {
			for i := 0; i < 10_000_000; i++ {
				sum1 += 1
				sum2 += 1
			}
		}

		fmt.Printf("thread %d: fin\n", tid)
	}

	go func() {
		t1 := thread.New()
		t1.CallNonBlock(func() {
			counter(t1.ID(), false)
		})
	}()

	t2 := thread.New()
	t2.CallNonBlock(func() {
		counter(t2.ID(), true)
	})

	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)
		fmt.Printf("sum1=%d, sum2=%d\n", sum1, sum2)
	}
}

func main() {
	testAtomic()
}
