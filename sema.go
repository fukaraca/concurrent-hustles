// Question 5: Semaphore Implementation
// Implement a semaphore using channels to limit concurrent access

package main

import (
	"fmt"
	"sync"
	"time"
)

type Semaphore struct {
	sem chan struct{}
}

// Create a semaphore with the specified number of permits
func NewSemaphore(maxConcurrent int) *Semaphore {
	sem := make(chan struct{}, maxConcurrent)
	for range maxConcurrent {
		sem <- struct{}{}
	}
	return &Semaphore{sem: sem}
}

// Acquire a permit (block if none available)
func (s *Semaphore) Acquire() {
	<-s.sem
}

// Release a permit
func (s *Semaphore) Release() {
	s.sem <- struct{}{}
}

// Try to acquire a permit without blocking
// Return true if acquired, false otherwise
func (s *Semaphore) TryAcquire() bool {

	select {
	case <-s.sem:
		return true
	default:
		return false
	}
}

func simulateWork(id int, sem *Semaphore, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("Task %d: Waiting for permit...\n", id)
	sem.Acquire()
	defer sem.Release()

	fmt.Printf("Task %d: Got permit, working...\n", id)
	time.Sleep(500 * time.Millisecond)
	fmt.Printf("Task %d: Done\n", id)
}

func main() {
	fmt.Println("=== Test 1: Basic semaphore with 2 permits ===")
	sem := NewSemaphore(2)
	var wg sync.WaitGroup

	start := time.Now()
	// Launch 6 tasks, but only 2 can run concurrently
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go simulateWork(i, sem, &wg)
	}
	wg.Wait()
	elapsed := time.Since(start)
	fmt.Printf("Completed in %v (should be ~1.5s)\n\n", elapsed)

	fmt.Println("=== Test 2: TryAcquire ===")
	sem2 := NewSemaphore(1)
	if sem2.TryAcquire() {
		fmt.Println("First TryAcquire: Success")
	}
	if !sem2.TryAcquire() {
		fmt.Println("Second TryAcquire: Failed (expected)")
	}
	sem2.Release()
	if sem2.TryAcquire() {
		fmt.Println("Third TryAcquire: Success")
	}
}

// Test: Run with `go run main.go` and `go test -race`
// Expected: Only 2 tasks run concurrently, total time ~1.5s
// TryAcquire should succeed/fail as expected
// No race conditions
