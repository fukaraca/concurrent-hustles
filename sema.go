// Semaphore Implementation
// Implement a semaphore using channels to limit concurrent access

package concurrent_hustles

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
