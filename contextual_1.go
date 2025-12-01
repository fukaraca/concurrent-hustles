// Question 4: Context-based Cancellation
// Implement a task that can be cancelled using context

package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type TaskResult struct {
	ID        int
	Completed bool
	Value     int
}

// This task should:
// 1. Process for the specified duration
// 2. Respect context cancellation
// 3. Send progress updates every 100ms
// 4. Return a result when completed or cancelled
func longRunningTask(ctx context.Context, id int, duration time.Duration) TaskResult {
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	start := time.Now()
	out := TaskResult{ID: id}
	for {
		elapsed := time.Since(start)
		select {
		case <-ticker.C:
			fmt.Printf("tick id %d\n", id)
			out.Value = int(elapsed.Milliseconds())
			if time.Since(start) >= duration {
				out.Completed = true
				return out
			}
		case <-ctx.Done():
			out.Value = int(elapsed.Milliseconds())
			return out
		}
	}
}

// Run multiple tasks concurrently with a timeout
// Cancel all tasks if timeout is reached
// Return all results (completed or cancelled)
func runWithTimeout(timeout time.Duration, numTasks int, taskDuration time.Duration) []TaskResult {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	start := time.Now()
	results := make([]TaskResult, numTasks)
	var wg sync.WaitGroup
	for i := 1; i <= numTasks; i++ {
		wg.Go(func() {
			results[i-1] = longRunningTask(ctx, i, taskDuration)
		})
	}
	wg.Wait()
	fmt.Printf("runWith took %d millseconds \n", time.Since(start).Milliseconds())
	return results
}

func main() {
	fmt.Println("=== Test 1: Tasks complete before timeout ===")
	results1 := runWithTimeout(2*time.Second, 3, 500*time.Millisecond)
	for _, result := range results1 {
		fmt.Printf("Task %d: Completed=%v, Value=%d\n", result.ID, result.Completed, result.Value)
	}

	fmt.Println("\n=== Test 2: Tasks cancelled by timeout ===")
	results2 := runWithTimeout(1*time.Second, 3, 3*time.Second)
	for _, result := range results2 {
		fmt.Printf("Task %d: Completed=%v, Value=%d\n", result.ID, result.Completed, result.Value)
	}
}

// Test: Run with `go run main.go` and `go test -race`
// Expected Test 1: All 3 tasks should complete (Completed=true)
// Expected Test 2: All 3 tasks should be cancelled (Completed=false)
// No goroutine leaks, no race conditions
