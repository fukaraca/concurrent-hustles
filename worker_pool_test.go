package concurrent_hustles

import (
	"sync"
	"testing"
	"time"
)

// Test basic worker pool functionality
func TestWorkerPool_BasicOperation(t *testing.T) {
	pool := NewWorkerPool(3)
	pool.Start()

	numJobs := 10
	go func() {
		for i := 0; i < numJobs; i++ {
			pool.Submit(Job{
				ID:    i + 1,
				Value: i,
			})
		}
		close(pool.jobs)
	}()

	// Collect results
	results := make(map[int]int)
	go pool.Stop()
	for r := range pool.results {
		results[r.ID] = r.Val
	}

	// Verify all jobs completed
	if len(results) != numJobs {
		t.Errorf("Expected %d results, got %d", numJobs, len(results))
	}

	// Verify correct calculations
	for i := 0; i < numJobs; i++ {
		jobID := i + 1
		expected := i * i
		if results[jobID] != expected {
			t.Errorf("Job %d: expected %d, got %d", jobID, expected, results[jobID])
		}
	}
}

// Test with single worker
func TestWorkerPool_SingleWorker(t *testing.T) {
	pool := NewWorkerPool(1)
	pool.Start()

	numJobs := 5
	go func() {
		for i := 0; i < numJobs; i++ {
			pool.Submit(Job{ID: i, Value: i})
		}
		close(pool.jobs)
	}()

	results := make(map[int]int)
	go pool.Stop()
	for r := range pool.results {
		results[r.ID] = r.Val
	}

	if len(results) != numJobs {
		t.Errorf("Expected %d results, got %d", numJobs, len(results))
	}
}

// Test with many workers
func TestWorkerPool_ManyWorkers(t *testing.T) {
	pool := NewWorkerPool(10)
	pool.Start()

	numJobs := 20
	go func() {
		for i := 0; i < numJobs; i++ {
			pool.Submit(Job{ID: i, Value: i})
		}
		close(pool.jobs)
	}()

	results := make(map[int]int)
	go pool.Stop()
	for r := range pool.results {
		results[r.ID] = r.Val
	}

	if len(results) != numJobs {
		t.Errorf("Expected %d results, got %d", numJobs, len(results))
	}
}

// Test empty job queue
func TestWorkerPool_NoJobs(t *testing.T) {
	pool := NewWorkerPool(3)
	pool.Start()

	close(pool.jobs)
	go pool.Stop()

	count := 0
	for range pool.results {
		count++
	}

	if count != 0 {
		t.Errorf("Expected 0 results, got %d", count)
	}
}

// Test concurrent job submission
func TestWorkerPool_ConcurrentSubmission(t *testing.T) {
	pool := NewWorkerPool(5)
	pool.Start()

	numJobs := 100
	var wg sync.WaitGroup

	// Submit jobs from multiple goroutines
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				jobID := offset*20 + i
				pool.Submit(Job{ID: jobID, Value: jobID})
			}
		}(g)
	}

	go func() {
		wg.Wait()
		close(pool.jobs)
	}()

	results := make(map[int]int)
	go pool.Stop()
	for r := range pool.results {
		results[r.ID] = r.Val
	}

	if len(results) != numJobs {
		t.Errorf("Expected %d results, got %d", numJobs, len(results))
	}

	// Verify each result
	for id, val := range results {
		expected := id * id
		if val != expected {
			t.Errorf("Job %d: expected %d, got %d", id, expected, val)
		}
	}
}

// Test that all workers are actually working
func TestWorkerPool_AllWorkersActive(t *testing.T) {
	numWorkers := 5
	pool := NewWorkerPool(numWorkers)
	pool.Start()

	// Submit more jobs than workers to ensure all workers get tasks
	numJobs := numWorkers * 3
	go func() {
		for i := 0; i < numJobs; i++ {
			pool.Submit(Job{ID: i, Value: i})
		}
		close(pool.jobs)
	}()

	results := make(map[int]int)
	go pool.Stop()
	for r := range pool.results {
		results[r.ID] = r.Val
	}

	if len(results) != numJobs {
		t.Errorf("Expected %d results, got %d", numJobs, len(results))
	}
}

// Test correct calculation of squares
func TestWorkerPool_CorrectCalculations(t *testing.T) {
	pool := NewWorkerPool(3)
	pool.Start()

	testCases := []struct {
		id    int
		value int
		want  int
	}{
		{1, 0, 0},
		{2, 1, 1},
		{3, 2, 4},
		{4, 5, 25},
		{5, 10, 100},
		{6, -3, 9},
		{7, -5, 25},
	}

	go func() {
		for _, tc := range testCases {
			pool.Submit(Job{ID: tc.id, Value: tc.value})
		}
		close(pool.jobs)
	}()

	results := make(map[int]int)
	go pool.Stop()
	for r := range pool.results {
		results[r.ID] = r.Val
	}

	for _, tc := range testCases {
		if got := results[tc.id]; got != tc.want {
			t.Errorf("Job %d with value %d: expected %d^2=%d, got %d",
				tc.id, tc.value, tc.value, tc.want, got)
		}
	}
}

// Benchmark worker pool performance
func BenchmarkWorkerPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		pool := NewWorkerPool(5)
		pool.Start()

		go func() {
			for j := 0; j < 100; j++ {
				pool.Submit(Job{ID: j, Value: j})
			}
			close(pool.jobs)
		}()

		go pool.Stop()
		for range pool.results {
			// Consume results
		}
	}
}

// Test with large number of jobs
func TestWorkerPool_LargeJobCount(t *testing.T) {
	pool := NewWorkerPool(10)
	pool.Start()

	numJobs := 1000
	go func() {
		for i := 0; i < numJobs; i++ {
			pool.Submit(Job{ID: i, Value: i % 100})
		}
		close(pool.jobs)
	}()

	count := 0
	go pool.Stop()
	for range pool.results {
		count++
	}

	if count != numJobs {
		t.Errorf("Expected %d results, got %d", numJobs, count)
	}
}

// Test rapid start/stop cycles
func TestWorkerPool_RapidCycles(t *testing.T) {
	for cycle := 0; cycle < 5; cycle++ {
		pool := NewWorkerPool(3)
		pool.Start()

		go func() {
			for i := 0; i < 10; i++ {
				pool.Submit(Job{ID: i, Value: i})
			}
			close(pool.jobs)
		}()

		results := 0
		go pool.Stop()
		for range pool.results {
			results++
		}

		if results != 10 {
			t.Errorf("Cycle %d: expected 10 results, got %d", cycle, results)
		}
	}
}

// Test timeout scenario - ensure workers don't hang
func TestWorkerPool_WithTimeout(t *testing.T) {
	pool := NewWorkerPool(3)
	pool.Start()

	done := make(chan bool)
	go func() {
		for range pool.results {
			// Consume results
		}
		done <- true
	}()
	for i := 0; i < 5; i++ {
		pool.Submit(Job{ID: i, Value: i})
	}
	close(pool.jobs)
	go pool.Stop()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Worker pool hung and didn't complete within timeout")
	}
}
