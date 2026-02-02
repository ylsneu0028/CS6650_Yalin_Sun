package main

import (
	"fmt"
	"sync"
	"time"
)

// SafeMap wraps a regular map with a mutex
// to make all accesses to the map thread-safe.
type SafeMap struct {
	// mu protects concurrent access to the map
	mu sync.Mutex

	// m stores the actual key-value data
	m map[int]int
}

// NewSafeMap initializes and returns a new SafeMap instance.
func NewSafeMap() *SafeMap {
	return &SafeMap{
		m: make(map[int]int),
	}
}

// Set safely inserts or updates a key-value pair in the map.
// The mutex ensures that only one goroutine can write at a time.
func (s *SafeMap) Set(key, val int) {
	s.mu.Lock()         // Acquire the lock before modifying the map
	s.m[key] = val      // Write to the map
	s.mu.Unlock()       // Release the lock
}

// Len safely returns the number of elements in the map.
// The mutex prevents concurrent read/write conflicts.
func (s *SafeMap) Len() int {
	s.mu.Lock()         // Acquire the lock before reading the map
	n := len(s.m)       // Read the map size
	s.mu.Unlock()       // Release the lock
	return n
}

func main() {
	// Create a thread-safe map
	sm := NewSafeMap()

	// WaitGroup is used to wait for all goroutines to complete
	var wg sync.WaitGroup

	// Record the start time to measure total execution time
	start := time.Now()

	// Launch 50 goroutines
	for g := 0; g < 50; g++ {

		// Increment the WaitGroup counter for each goroutine
		wg.Add(1)

		// Start a goroutine, passing g as an argument to avoid closure capture issues
		go func(g int) {

			// Ensure the WaitGroup counter is decremented when the goroutine finishes
			defer wg.Done()

			// Each goroutine inserts 1000 entries into the SafeMap
			for i := 0; i < 1000; i++ {

				// Safely write to the shared map using the mutex-protected Set method
				sm.Set(g*1000+i, i)
			}
		}(g)
	}

	// Wait until all goroutines have finished execution
	wg.Wait()

	// Measure the total elapsed time
	elapsed := time.Since(start)

	// Print the number of elements in the map
	// Expected value: 50 * 1000 = 50000
	fmt.Println("len(m):", sm.Len())

	// Print the total execution time
	fmt.Println("total time:", elapsed)
}

