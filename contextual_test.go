package concurrent_hustles

import (
	"testing"
	"time"
)

// TestTasksComplete corresponds to "Test 1" from the original main function.
// It verifies that tasks finish successfully if the timeout is longer than the task duration.
func TestTasksComplete(t *testing.T) {
	timeout := 2 * time.Second
	numTasks := 3
	taskDuration := 500 * time.Millisecond

	t.Logf("Running %d tasks of %v with timeout %v", numTasks, taskDuration, timeout)
	results := runWithTimeout(timeout, numTasks, taskDuration)

	if len(results) != numTasks {
		t.Fatalf("Expected %d results, got %d", numTasks, len(results))
	}

	for _, result := range results {
		if !result.Completed {
			t.Errorf("Task %d expected Completed=true, got false", result.ID)
		}

		// Allow a small margin of error for timing (e.g., +/- 100ms)
		// The value should be roughly equal to the task duration
		if result.Value < 400 || result.Value > 700 {
			t.Logf("Warning: Task %d value %d ms is outside expected range, but completed", result.ID, result.Value)
		}
	}
}

// TestTasksCancelled corresponds to "Test 2" from the original main function.
// It verifies that tasks are cancelled if the timeout is shorter than the task duration.
func TestTasksCancelled(t *testing.T) {
	timeout := 1 * time.Second
	numTasks := 3
	taskDuration := 3 * time.Second

	t.Logf("Running %d tasks of %v with timeout %v", numTasks, taskDuration, timeout)
	results := runWithTimeout(timeout, numTasks, taskDuration)

	if len(results) != numTasks {
		t.Fatalf("Expected %d results, got %d", numTasks, len(results))
	}

	for _, result := range results {
		if result.Completed {
			t.Errorf("Task %d expected Completed=false (cancelled), got true", result.ID)
		}

		// The value should be roughly equal to the timeout (1000ms)
		// Since tickers tick every 100ms, we expect it to be close to 1000ms
		if result.Value < 900 || result.Value > 1100 {
			t.Logf("Warning: Task %d value %d ms is outside expected cancel range", result.ID, result.Value)
		}
	}
}

// TestZeroTimeout is an edge case test
func TestZeroTimeout(t *testing.T) {
	timeout := 0 * time.Second
	numTasks := 1
	taskDuration := 1 * time.Second

	results := runWithTimeout(timeout, numTasks, taskDuration)

	if results[0].Completed {
		t.Error("Task should have been cancelled immediately with 0 timeout")
	}
}
