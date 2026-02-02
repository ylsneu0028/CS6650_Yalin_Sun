package main

import (
	"fmt"
	"runtime"
	"time"
)

const N = 1_000_000 // Number of ping-pong iterations

// pingPongOnce measures the time taken for n ping-pong
// handoffs between two goroutines using an unbuffered channel.
func pingPongOnce(n int) time.Duration {

	// ch is an unbuffered channel used for synchronization.
	// Each send must wait for a corresponding receive.
	ch := make(chan struct{})

	// done is used to signal completion of goroutine B
	done := make(chan struct{})

	// Goroutine B:
	// Waits to receive a signal from ch, then sends a response back.
	go func() {
		for i := 0; i < n; i++ {
			<-ch              // Wait for signal from goroutine A
			ch <- struct{}{}  // Send response back to goroutine A
		}
		close(done) // Signal completion
	}()

	// Start timing right before the ping-pong loop begins
	start := time.Now()

	// Goroutine A (main goroutine):
	// Sends a signal, then waits for a response.
	for i := 0; i < n; i++ {
		ch <- struct{}{} // Send signal to goroutine B
		<-ch             // Wait for response
	}

	// Wait until goroutine B finishes
	<-done

	// Return total elapsed time
	return time.Since(start)
}

// runCase sets GOMAXPROCS, runs the ping-pong benchmark,
// and prints total time and average handoff cost.
func runCase(name string, procs int) {

	// Set the maximum number of OS threads that can execute Go code simultaneously
	prev := runtime.GOMAXPROCS(procs)
	_ = prev // Previous value is ignored but kept for clarity

	// Run the ping-pong benchmark
	d := pingPongOnce(N)

	// Each iteration performs two channel handoffs (send + receive),
	// so divide total time by 2*N to get average time per handoff
	avg := d.Seconds() / float64(2*N)

	// Print results
	fmt.Printf("%s: total=%v, avg_handoff=%.2f ns\n", name, d, avg*1e9)
}

func main() {
	// Case A: restrict Go scheduler to a single OS thread
	// Goroutines are multiplexed onto one thread
	runCase("GOMAXPROCS=1", 1)

	// Case B: allow Go to use all available CPU cores
	// Goroutines may run in parallel on multiple OS threads
	runCase("GOMAXPROCS=NumCPU", runtime.NumCPU())
}

