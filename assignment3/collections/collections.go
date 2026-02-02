package main

import (
    "fmt"
    "sync"
)

func main() {

    // Create a map that stores integer key-value pairs
    // This map will be shared by multiple goroutines
    m := make(map[int]int)

    // WaitGroup is used to wait for all goroutines to complete
    var wg sync.WaitGroup

    // Launch 50 goroutines
    for g := 0; g < 50; g++ {

        // Increment the WaitGroup counter for each goroutine
        wg.Add(1)

        // Start a goroutine, passing g as an argument to avoid closure issues
        go func(g int) {

            // Ensure the WaitGroup counter is decremented when the goroutine finishes
            defer wg.Done()

            // Each goroutine performs 1000 write operations on the map
            for i := 0; i < 1000; i++ {

                // Write a key-value pair into the shared map
                // Key is unique per goroutine (g*1000 + i)
                // Value is the current loop index i
                // Note: concurrent writes to a map without synchronization
                // can cause a race condition or runtime panic
                m[g*1000+i] = i
            }
        }(g)
    }

    // Wait until all goroutines have finished
    wg.Wait()

    // Print the number of entries in the map
    fmt.Println("len(m):", len(m))
}

