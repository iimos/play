package main

import (
	"sync/atomic"
	"testing"
	"unsafe"

	"golang.org/x/sys/cpu"
)

// https://github.com/golang/go/issues/25203

const cacheLineSize = unsafe.Sizeof(cpu.CacheLinePad{}) // bytes

// here a and b lives in a different cache lines
type paddingYes struct {
	a uint32
	_ [cacheLineSize - unsafe.Sizeof(uint32(0))]byte
	b uint32
}

// here a and b share common cache line
type paddingNo struct {
	a uint32
	b uint32
}

func BenchmarkFalseSharing_NoPadding(t *testing.B) {
	s := paddingNo{}

	done := make(chan struct{})
	go func() {
		for i := 0; i < t.N; i++ {
			atomic.AddUint32(&s.a, 1)
		}
		close(done)
	}()

	for i := 0; i < t.N; i++ {
		atomic.AddUint32(&s.b, 1)
	}

	<-done

	// fmt.Printf("- {a=%d;\t b=%d}\t (N=%d)\n", s.a, s.b, t.N)
}

func BenchmarkFalseSharing_WithPadding(t *testing.B) {
	s := paddingYes{}

	done := make(chan struct{})
	go func() {
		for i := 0; i < t.N; i++ {
			atomic.AddUint32(&s.a, 1)
		}
		close(done)
	}()

	for i := 0; i < t.N; i++ {
		atomic.AddUint32(&s.b, 1)
	}

	<-done

	// fmt.Printf("- {a=%d;\t b=%d}\t (N=%d)\n", s.a, s.b, t.N)
}
