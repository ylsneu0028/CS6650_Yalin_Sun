package main

import (
    "fmt"
    "sync"
)

func main() {

    // ops is a shared counter that records the total number of operations
    var ops uint64

    // wg is a WaitGroup used to wait for all goroutines to finish
    var wg sync.WaitGroup

    // Start 50 concurrent goroutines
    for range 50 {

        // Launch a goroutine and register it with the WaitGroup
        wg.Go(func() {

            // Each goroutine increments the counter 1000 times
            for range 1000 {

                // Increment the shared counter
                // This operation is not atomic and may cause a race condition
                ops++
            }
        })
    }

    // Wait until all goroutines have completed
    wg.Wait()

    // Print the final value of the counter
    fmt.Println("ops:", ops)
}

