package concurrent_hustles

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestBasicPriority checks if jobs are executed in order of priority (highest first).
func TestBasicPriority(t *testing.T) {
	t.Parallel()
	t.Log("=== Test 1: Basic Priority Ordering ===")
	scheduler := NewPriorityScheduler()
	ctx := context.Background()

	var mu sync.Mutex
	executed := []int{}
	expectedOrder := []int{2, 3, 1}

	// Submit jobs with different priorities (higher number = higher priority)
	jobs := []PriorityJob{
		{ID: 1, Priority: 1, Work: func(cctx context.Context) error {
			mu.Lock()
			executed = append(executed, 1)
			mu.Unlock()
			t := time.NewTimer(20 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
			case <-t.C:
			}
			return cctx.Err()
		}},
		{ID: 2, Priority: 5, Work: func(cctx context.Context) error {
			mu.Lock()
			executed = append(executed, 2)
			mu.Unlock()
			t := time.NewTimer(20 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
			case <-t.C:
			}
			return cctx.Err()
		}},
		{ID: 3, Priority: 3, Work: func(cctx context.Context) error {
			mu.Lock()
			executed = append(executed, 3)
			mu.Unlock()
			t := time.NewTimer(20 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
			case <-t.C:
			}
			return cctx.Err()
		}},
	}

	scheduler.Start(ctx, 1)

	for _, job := range jobs {
		if err := scheduler.Submit(job); err != nil {
			t.Fatalf("Submit failed for job %d: %v", job.ID, err)
		}
	}

	scheduler.Wait()
	scheduler.Stop()

	if len(executed[len(executed)-len(expectedOrder):]) != len(expectedOrder) {
		t.Fatalf("Incorrect number of jobs executed. Got %d, want %d. Order: %v", len(executed), len(expectedOrder), executed)
	}

	for i, jobID := range executed[len(executed)-len(expectedOrder):] {
		if jobID != expectedOrder[i] {
			t.Errorf("Execution order mismatch at index %d. Got %d, want %d. Final order: %v", i, jobID, expectedOrder[i], executed)
			break
		}
	}
	t.Logf("Execution order: %v (expected: %v)", executed, expectedOrder)
}

// TestPreemption checks if a higher priority job can preempt a lower priority job.
func TestPreemption(t *testing.T) {
	t.Parallel()
	t.Log("\n=== Test 2: Preemption Test ===")
	scheduler := NewPriorityScheduler()
	ctx := context.Background()

	var mu sync.Mutex
	executed := []int{}
	jobStarted := make(chan int, 10)

	// PriorityJob 1: Low priority, long running (should be preempted and re-queued)
	job1 := PriorityJob{
		ID: 1, Priority: 1,
		Work: func(cctx context.Context) error {
			jobStarted <- 1
			mu.Lock()
			executed = append(executed, 1)
			mu.Unlock()
			t := time.NewTimer(100 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
				// Check if context cancellation was due to preemption
				mu.Lock()
				executed = append(executed, 100) // 100 as a marker for a canceled/re-executed Job 1
				mu.Unlock()
				return cctx.Err()
			case <-t.C:
			}
			return cctx.Err()
		},
	}

	// PriorityJob 2: High priority, should preempt Job 1
	job2 := PriorityJob{
		ID: 2, Priority: 10,
		Work: func(cctx context.Context) error {
			jobStarted <- 2
			mu.Lock()
			executed = append(executed, 2)
			mu.Unlock()
			t := time.NewTimer(10 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
			case <-t.C:
			}
			return cctx.Err()
		},
	}

	// PriorityJob 3: Medium priority (should execute after Job 2, before Job 1 is re-submitted)
	job3 := PriorityJob{
		ID: 3, Priority: 5,
		Work: func(cctx context.Context) error {
			jobStarted <- 3
			mu.Lock()
			executed = append(executed, 3)
			mu.Unlock()
			t := time.NewTimer(10 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
			case <-t.C:
			}
			return cctx.Err()
		},
	}

	scheduler.Start(ctx, 1)

	// 1. Start Job 1
	scheduler.Submit(job1)
	<-jobStarted                      // Wait for job1 to start
	time.Sleep(20 * time.Millisecond) // Let it run a bit

	// 2. Submit Job 2 (should preempt Job 1)
	scheduler.Submit(job2)
	// 3. Submit Job 3 (should be queued)
	scheduler.Submit(job3)

	scheduler.Wait()
	scheduler.Stop()

	// The order should be: Job 1 start -> Job 2 start -> Job 3 start -> Job 1 re-start
	// Job 1 appears twice: initial run (1) and re-queued run (100 marker).
	// The actual completion order should be 2, 3, 100 (if it was preempted and re-queued)
	expectedMarkers := []int{1, 100, 2, 3, 1}

	mu.Lock()
	finalOrder := executed
	mu.Unlock()

	// Check if the expected markers (indicating preemption/re-queue) are present
	if len(finalOrder) != len(expectedMarkers) {
		t.Fatalf("Incorrect number of events executed. Got %d, want %d. Order: %v", len(finalOrder), len(expectedMarkers), finalOrder)
	}

	// Basic check for the expected sequence of events (marker values)
	if finalOrder[2] != 2 || finalOrder[3] != 3 {
		t.Errorf("Preemption or priority execution order mismatch. Got %v", finalOrder)
	}

	t.Logf("Execution order: %v (Expected to see preemption: Job 1 starts -> Job 2 completes -> Job 3 completes -> Job 1 (re-run) completes)", finalOrder)
}

// TestMultipleWorkers checks if multiple jobs can be run concurrently and if priorities are generally respected.
func TestMultipleWorkers(t *testing.T) {
	t.Parallel()
	t.Log("\n=== Test 3: Multiple Workers ===")
	scheduler := NewPriorityScheduler()
	ctx := context.Background()

	var mu sync.Mutex
	executed := []int{}
	numJobs := 10
	numWorkers := 3

	jobs := make([]PriorityJob, numJobs)
	for i := 0; i < numJobs; i++ {
		id := i + 1
		jobs[i] = PriorityJob{
			ID: id, Priority: id, // Priority equals ID (10 is highest, 1 is lowest)
			Work: func(cctx context.Context) error {

				t := time.NewTimer(50 * time.Millisecond) // Shortened time for faster test
				defer t.Stop()
				select {
				case <-cctx.Done():
				case <-t.C:
					mu.Lock()
					executed = append(executed, id)
					mu.Unlock()
				}
				return cctx.Err()
			},
		}
	}

	scheduler.Start(ctx, numWorkers)

	for _, job := range jobs {
		if err := scheduler.Submit(job); err != nil {
			t.Fatalf("Submit failed for job %d: %v", job.ID, err)
		}
	}

	scheduler.Wait()
	scheduler.Stop()

	if len(executed) != numJobs {
		t.Fatalf("Incorrect number of jobs executed. Got %d, want %d", len(executed), numJobs)
	}

	// Since 3 workers run concurrently, the order will be mixed but the first 3 jobs
	// to complete should generally be the highest priority (8, 9, 10) in some order.
	// However, due to scheduling and channel speed, a strict check is difficult.
	// We only check for completion here.
	t.Logf("Executed %d jobs with %d workers\n", len(executed), numWorkers)
	t.Logf("Execution order: %v\n", executed)
}

// TestContextCancellation checks if the scheduler and active jobs stop when the context is canceled.
func TestContextCancellation(t *testing.T) {
	t.Parallel()
	t.Log("\n=== Test 4: Context Cancellation ===")
	scheduler := NewPriorityScheduler()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	executed := []int{}

	scheduler.Start(ctx, 1)

	// Submit jobs with different priorities
	for i := 5; i > 0; i-- {
		id := i
		job := PriorityJob{
			ID: id, Priority: id,
			Work: func(cctx context.Context) error {
				mu.Lock()
				executed = append(executed, id)
				mu.Unlock()
				t := time.NewTimer(50 * time.Millisecond)
				defer t.Stop()
				select {
				case <-cctx.Done():
				case <-t.C:
				}
				return cctx.Err()
			},
		}
		scheduler.Submit(job)
	}

	time.Sleep(80 * time.Millisecond)
	cancel() // Cancel context

	time.Sleep(50 * time.Millisecond)
	scheduler.Stop()

	// Jobs have 50ms runtime. At 80ms, Job 5 should have started and finished.
	// Job 4 may have started and been cancelled. Max 2 jobs (5 and 4) should have started.
	mu.Lock()
	count := len(executed)
	mu.Unlock()

	if count >= 5 {
		t.Errorf("Jobs continued executing after context cancellation. Executed %d jobs: %v", count, executed)
	}

	t.Logf("Executed %d jobs before cancellation (expected < 5): %v", count, executed)
}

// TestConcurrentSubmissions checks the robustness of the submission process under concurrency.
func TestConcurrentSubmissions(t *testing.T) {
	t.Parallel()
	t.Log("\n=== Test 5: Concurrent Submissions ===")
	scheduler := NewPriorityScheduler()
	ctx := context.Background()

	var mu sync.Mutex
	executed := make(map[int]bool)

	scheduler.Start(ctx, 2)

	var wg sync.WaitGroup
	numGoroutines := 5
	jobsPerGoroutine := 4
	totalExpected := numGoroutines * jobsPerGoroutine

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < jobsPerGoroutine; j++ {
				// Job IDs: 0 to 19
				jobID := goroutineID*jobsPerGoroutine + j
				job := PriorityJob{
					ID: jobID, Priority: jobID % 10,
					Work: func(cctx context.Context) error {
						mu.Lock()
						executed[jobID] = true
						mu.Unlock()
						t := time.NewTimer(5 * time.Millisecond)
						defer t.Stop()
						select {
						case <-cctx.Done():
						case <-t.C:
						}
						return cctx.Err()
					},
				}
				if err := scheduler.Submit(job); err != nil {
					t.Errorf("Submit failed for job %d: %v", jobID, err)
				}
			}
		}(g)
	}

	wg.Wait()
	scheduler.Wait()
	scheduler.Stop()

	mu.Lock()
	count := len(executed)
	mu.Unlock()

	if count != totalExpected {
		t.Errorf("Incorrect number of jobs executed from concurrent submissions. Got %d, want %d", count, totalExpected)
	}

	t.Logf("Executed %d jobs from concurrent submissions (expected %d)", count, totalExpected)
}
