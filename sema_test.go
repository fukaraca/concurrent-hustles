package concurrent_hustles

import (
	"sync"
	"testing"
	"time"
)

func TestBasicSemaphore(t *testing.T) {
	t.Log("=== Test 1: Basic semaphore with 2 permits ===")
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

	t.Logf("Completed in %v", elapsed)

	// Should take approximately 1.5s (3 batches of 2 tasks, each taking 500ms)
	minExpected := 1400 * time.Millisecond
	maxExpected := 1700 * time.Millisecond

	if elapsed < minExpected || elapsed > maxExpected {
		t.Errorf("Expected duration between %v and %v, got %v", minExpected, maxExpected, elapsed)
	}
}

func TestTryAcquire(t *testing.T) {
	t.Log("=== Test 2: TryAcquire ===")
	sem := NewSemaphore(1)

	// First TryAcquire should succeed
	if !sem.TryAcquire() {
		t.Error("First TryAcquire: Expected success, got failure")
	} else {
		t.Log("First TryAcquire: Success")
	}

	// Second TryAcquire should fail (no permits available)
	if sem.TryAcquire() {
		t.Error("Second TryAcquire: Expected failure, got success")
	} else {
		t.Log("Second TryAcquire: Failed (expected)")
	}

	// Release the permit
	sem.Release()

	// Third TryAcquire should succeed
	if !sem.TryAcquire() {
		t.Error("Third TryAcquire: Expected success, got failure")
	} else {
		t.Log("Third TryAcquire: Success")
	}
}

func TestSemaphoreConcurrency(t *testing.T) {
	t.Log("=== Test 3: Verify concurrent access limit ===")
	maxConcurrent := 3
	sem := NewSemaphore(maxConcurrent)
	var wg sync.WaitGroup

	concurrentCount := 0
	maxObserved := 0
	var mu sync.Mutex

	numTasks := 10
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			sem.Acquire()
			defer sem.Release()

			mu.Lock()
			concurrentCount++
			if concurrentCount > maxObserved {
				maxObserved = concurrentCount
			}
			current := concurrentCount
			mu.Unlock()

			if current > maxConcurrent {
				t.Errorf("Task %d: Too many concurrent tasks: %d (max: %d)", id, current, maxConcurrent)
			}

			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			concurrentCount--
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	t.Logf("Max concurrent tasks observed: %d (limit: %d)", maxObserved, maxConcurrent)

	if maxObserved > maxConcurrent {
		t.Errorf("Exceeded maximum concurrent limit: observed %d, limit %d", maxObserved, maxConcurrent)
	}
}

func TestSemaphoreZeroPermits(t *testing.T) {
	t.Log("=== Test 4: Semaphore with zero permits ===")
	sem := NewSemaphore(0)

	// TryAcquire should always fail
	if sem.TryAcquire() {
		t.Error("TryAcquire with zero permits should fail")
	}

	// Release a permit
	go sem.Release()
	time.Sleep(5 * time.Millisecond) // wait for goroutine

	// Now TryAcquire should succeed
	if !sem.TryAcquire() {
		t.Error("TryAcquire should succeed after Release")
	}
}

func TestSemaphoreMultipleReleases(t *testing.T) {
	t.Log("=== Test 5: Multiple releases ===")
	sem := NewSemaphore(2)

	// Acquire both permits
	sem.Acquire()
	sem.Acquire()

	// TryAcquire should fail
	if sem.TryAcquire() {
		t.Error("TryAcquire should fail when all permits are taken")
	}

	// Release one permit
	sem.Release()

	// TryAcquire should succeed
	if !sem.TryAcquire() {
		t.Error("TryAcquire should succeed after one Release")
	}

	// Release another permit
	sem.Release()

	// TryAcquire should succeed
	if !sem.TryAcquire() {
		t.Error("TryAcquire should succeed after second Release")
	}
}
