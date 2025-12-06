// Worker Pool Pattern
// Implement a worker pool that processes jobs concurrently.
// The pool should have a fixed number of workers and handle jobs from a queue.

package concurrent_hustles

import (
	"fmt"
	"sync"
	"time"
)

type Job struct {
	ID    int
	Value int
}

type Result struct {
	ID, Val int
}

type WorkerPool struct {
	numWorkers int
	jobs       chan Job
	results    chan Result
	wg         sync.WaitGroup
}

// Create a new worker pool with the specified number of workers
func NewWorkerPool(numWorkers int) *WorkerPool {
	// YOUR CODE HERE
	return &WorkerPool{
		numWorkers: numWorkers,
		jobs:       make(chan Job),
		results:    make(chan Result),
		wg:         sync.WaitGroup{},
	}
}

// Start all workers in the pool
func (wp *WorkerPool) Start() {
	for i := range wp.numWorkers {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Each worker should process jobs from the jobs channel
// For each job, calculate the square of the value and send it to the result channel
func (wp *WorkerPool) worker(i int) {
	defer wp.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case c, ok := <-wp.jobs:
			if !ok {
				fmt.Printf("exiting worker %d\n", i)
				return
			}
			fmt.Printf("job %d processed in worker %d\n", c.ID, i)
			wp.results <- Result{c.ID, c.Value * c.Value}
		case <-ticker.C:
			fmt.Printf("tickk %d\n", i)
		}
	}
}

// Submit a job to the worker pool
func (wp *WorkerPool) Submit(job Job) {
	wp.jobs <- job
}

// Stop the worker pool and wait for all workers to finish
func (wp *WorkerPool) Stop() {
	wp.wg.Wait()
	close(wp.results)
}
