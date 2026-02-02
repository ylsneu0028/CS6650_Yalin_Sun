package main

import (
	"fmt"
	"sync"
	"time"
)

type SafeMap struct {
	mu sync.RWMutex
	m  map[int]int
}

func NewSafeMap() *SafeMap {
	return &SafeMap{m: make(map[int]int)}
}

func (s *SafeMap) Set(key, val int) {
	s.mu.Lock()
	s.m[key] = val
	s.mu.Unlock()
}

func (s *SafeMap) Len() int {
	s.mu.RLock()
	n := len(s.m)
	s.mu.RUnlock()
	return n
}

func main() {
	sm := NewSafeMap()

	var wg sync.WaitGroup
	start := time.Now()

	for g := 0; g < 50; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				sm.Set(g*1000+i, i)
			}
		}(g)
	}

	wg.Wait()
	elapsed := time.Since(start)

	fmt.Println("len(m):", sm.Len())
	fmt.Println("total time:", elapsed)
}
