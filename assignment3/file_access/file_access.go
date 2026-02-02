package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

const (
	// N is the number of lines to write to the file
	N = 100000

	// FileName is the base name of the output file
	FileName = "output.txt"
)

// writeUnbuffered writes data directly to the file without user-space buffering.
// Each Write call may result in a system call, which is relatively expensive.
func writeUnbuffered(path string, n int) (time.Duration, error) {

	// Create (or truncate) the file
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Reuse the same byte slice to avoid repeated allocations
	line := []byte("hello world\n")

	// Record the start time
	start := time.Now()

	// Write n lines directly to the file
	for i := 0; i < n; i++ {

		// Write sends data directly to the underlying file descriptor
		if _, err := f.Write(line); err != nil {
			return 0, err
		}
	}

	// Close flushes OS buffers to the kernel,
	// but does NOT guarantee the data is written to disk (no fsync)
	if err := f.Close(); err != nil {
		return 0, err
	}

	// Return the elapsed time
	return time.Since(start), nil
}

// writeBuffered writes data using a buffered writer.
// Data is first accumulated in user-space memory and flushed in larger chunks,
// reducing the number of system calls.
func writeBuffered(path string, n int) (time.Duration, error) {

	// Create (or truncate) the file
	f, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Create a buffered writer on top of the file
	w := bufio.NewWriter(f)

	// Use a string since WriteString avoids byte slice conversion
	line := "hello world\n"

	// Record the start time
	start := time.Now()

	// Write n lines into the buffer
	for i := 0; i < n; i++ {

		// WriteString appends data to the user-space buffer
		if _, err := w.WriteString(line); err != nil {
			return 0, err
		}
	}

	// Flush moves buffered data from user space to the underlying file (kernel)
	if err := w.Flush(); err != nil {
		return 0, err
	}

	// Close the file descriptor
	if err := f.Close(); err != nil {
		return 0, err
	}

	// Return the elapsed time
	return time.Since(start), nil
}

func main() {
	// Measure performance of unbuffered file writes
	d1, err := writeUnbuffered("unbuffered_"+FileName, N)
	if err != nil {
		fmt.Println("unbuffered error:", err)
		return
	}

	// Measure performance of buffered file writes
	d2, err := writeBuffered("buffered_"+FileName, N)
	if err != nil {
		fmt.Println("buffered error:", err)
		return
	}

	// Print the execution time for both approaches
	fmt.Println("unbuffered duration:", d1)
	fmt.Println("buffered   duration:", d2)
}

