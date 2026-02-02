package main

import (
	"bufio"
	"fmt"
	"os"
	"time"
)

const (
	N        = 100000
	FileName = "output.txt"
)

func writeUnbuffered(path string, n int) (time.Duration, error) {
	f, err := os.Create(path) // create/truncate
	if err != nil {
		return 0, err
	}
	defer f.Close()

	line := []byte("hello world\n") // reuse the same bytes to avoid allocation

	start := time.Now()
	for i := 0; i < n; i++ {
		if _, err := f.Write(line); err != nil {
			return 0, err
		}
	}
	// Close() will flush OS buffers to kernel, but not necessarily to disk (no fsync).
	if err := f.Close(); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

func writeBuffered(path string, n int) (time.Duration, error) {
	f, err := os.Create(path) // create/truncate
	if err != nil {
		return 0, err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	line := "hello world\n"

	start := time.Now()
	for i := 0; i < n; i++ {
		if _, err := w.WriteString(line); err != nil {
			return 0, err
		}
	}
	// Flush pushes data from user-space buffer to the underlying file (kernel).
	if err := w.Flush(); err != nil {
		return 0, err
	}
	if err := f.Close(); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

func main() {
	// Unbuffered
	d1, err := writeUnbuffered("unbuffered_"+FileName, N)
	if err != nil {
		fmt.Println("unbuffered error:", err)
		return
	}

	// Buffered
	d2, err := writeBuffered("buffered_"+FileName, N)
	if err != nil {
		fmt.Println("buffered error:", err)
		return
	}

	fmt.Println("unbuffered duration:", d1)
	fmt.Println("buffered   duration:", d2)
}
