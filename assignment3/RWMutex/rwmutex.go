package main

import (
	"fmt"
	"sync"
	"time"
)

// SafeMap is a thread-safe map implementation.
// It uses an RWMutex to allow concurrent reads
// while still protecting writes.
type SafeMap struct {
	// mu is a read-write mutex:
	// multiple readers can hold the lock at the same time,
	// but writers require exclusive access.
	mu sync.RWMutex

	// m stores the actual key-value pairs
	m map[int]int
}

// NewSafeMap creates and initializes a new SafeMap.
func NewSafeMap() *SafeMap {
	return &SafeMap{m: make(map[int]int)}
}

// Set inserts or updates a key-value pair in the map.
// A write lock is required because the map is being modified.
func (s *SafeMap) Set(key, val int) {
	s.mu.Lock()         // Acquire exclusive (write) lock
	s.m[key] = val      // Write to the map
	s.mu.Unlock()       // Release the write lock
}

// Len returns the number of elements in the map.
// A read lock is sufficient because the map is only being read.
func (s *SafeMap) Len() int {
	s.mu.RLock()        // Acquire shared (read) lock
	n := len(s.m)       // Read the map size
	s.mu.RUnlock()      // Release the read lock
	return n
}

func main() {
	// Create a thread-safe map
	sm := NewSafeMap()

	// WaitGroup is used to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Record the start time to measure execution duration
	start := time.Now()

	// Launch 50 goroutines
	for g := 0; g < 50; g++ {

		// Increment the WaitGroup counter for each goroutine
		wg.Add(1)

		// Start a goroutine, passing g as a parameter
		// to avoid closure capture issues
		go func(g int) {

			// Signal completion when the goroutine exits
			defer wg.Done()

			// Each goroutine writes 1000 entries into the map
			for i := 0; i < 1000; i++ {

				// Safely write to the shared map using a write lock
				sm.Set(g*1000+i, i)
			}
		}(g)
	}

	// Wait until all goroutines have completed
	wg.Wait()

	// Measure the total elapsed time
	elapsed := time.Since(start)

	// Print the number of elements in the map
	// Expected value: 50 * 1000 = 50000
	fmt.Println("len(m):", sm.Len())

	// Print the total execution time
	fmt.Println("total time:", elapsed)
}

