package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	// sync.Map is a concurrent map implementation provided by Go.
	// It is safe for concurrent use without explicit locking.
	var m sync.Map

	// WaitGroup is used to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Record the start time to measure total execution time
	start := time.Now()

	// Launch 50 goroutines
	for g := 0; g < 50; g++ {

		// Increment the WaitGroup counter for each goroutine
		wg.Add(1)

		// Start a goroutine, passing g as an argument
		// to avoid closure capture issues
		go func(g int) {

			// Signal completion when the goroutine finishes
			defer wg.Done()

			// Each goroutine stores 1000 key-value pairs
			// into the concurrent map
			for i := 0; i < 1000; i++ {

				// Store inserts or updates a key-value pair in sync.Map
				// This operation is safe for concurrent use
				m.Store(g*1000+i, i)
			}
		}(g)
	}

	// Wait until all goroutines have completed
	wg.Wait()

	// Count the number of entries in the map
	// Range iterates over the entire map, which is an O(n) operation
	count := 0
	m.Range(func(key, value any) bool {
		count++
		return true // continue iteration
	})

	// Measure the total elapsed time
	elapsed := time.Since(start)

	// Print the number of entries in the map
	// Expected value: 50 * 1000 = 50000
	fmt.Println("len(m):", count)

	// Print the total execution time
	fmt.Println("total time:", elapsed)
}

