package main

import (
	"fmt"
	"runtime"
	"time"
)

const N = 1_000_000

func pingPongOnce(n int) time.Duration {
	ch := make(chan struct{}) // unbuffered channel
	done := make(chan struct{})

	// Goroutine B: wait -> send back
	go func() {
		for i := 0; i < n; i++ {
			<-ch
			ch <- struct{}{}
		}
		close(done)
	}()

	// Start timing right before we begin the loop
	start := time.Now()

	// Goroutine A (main): send -> wait reply
	for i := 0; i < n; i++ {
		ch <- struct{}{}
		<-ch
	}

	<-done
	return time.Since(start)
}

func runCase(name string, procs int) {
	prev := runtime.GOMAXPROCS(procs)
	_ = prev // not used, but kept for clarity

	d := pingPongOnce(N)
	avg := d.Seconds() / float64(2*N) // seconds per handoff
	fmt.Printf("%s: total=%v, avg_handoff=%.2f ns\n", name, d, avg*1e9)
}

func main() {
	// Case A: single OS thread for Go scheduler
	runCase("GOMAXPROCS=1", 1)

	// Case B: allow Go to use multiple OS threads
	runCase("GOMAXPROCS=NumCPU", runtime.NumCPU())
}
