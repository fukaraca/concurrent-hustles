package concurrent_hustles

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestRunPipelineComplete verifies the pipeline runs to completion and produces the correct result.
func TestRunPipelineComplete(t *testing.T) {
	// Input N=10:
	// Stage 1 (Generate): 1, 2, 3, 4, 5, 6, 7, 8, 9, 10
	// Stage 2 (Filter Odd): 1, 3, 5, 7, 9
	// Stage 3 (Square): 1, 9, 25, 49, 81
	// Stage 4 (Sum): 1 + 9 + 25 + 49 + 81 = 165
	const expected = 165
	const inputN = 10

	ctx := context.Background()
	result, err := runPipeline(ctx, inputN)

	if err != nil {
		t.Fatalf("runPipeline(N=%d) failed unexpectedly with error: %v", inputN, err)
	}

	if result != expected {
		t.Errorf("runPipeline(N=%d) returned incorrect result.\nGot: %d\nWant: %d", inputN, result, expected)
	}
}

// TestRunPipelineCancelled verifies that the pipeline stops processing immediately
// when the context is cancelled, returning the expected context error.
func TestRunPipelineCancelled(t *testing.T) {
	// Use a large N to ensure the pipeline is busy when cancelled.
	const inputN = 1000000

	// Set up a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Create a channel to synchronize the pipeline execution start
	startCh := make(chan struct{})

	type pipelineResult struct {
		r int
		e error
	}
	resCh := make(chan pipelineResult)

	var result int
	var err error

	// Start the pipeline in a separate goroutine
	go func() {
		// Signal that the pipeline is running
		close(startCh)
		result, err = runPipeline(ctx, inputN)
		resCh <- pipelineResult{result, err}
		close(resCh)
	}()

	// Wait for the pipeline to start running
	<-startCh

	// Give it a very brief moment to get into the stages
	time.Sleep(1 * time.Millisecond)

	// Immediately cancel the context
	cancel()

	// Wait for a short time for the pipeline to process the cancellation signal
	// We'll use a timeout context to prevent the test from hanging indefinitely
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer timeoutCancel()

	// Wait on a channel that receives the result, but since we cancelled,
	// we just need to wait for the go routine to exit and rely on the error check.
	// Since we can't wait on the go routine easily without complex sync,
	// we rely on the time.Sleep and the error check below.

	// In a real testing scenario, we'd typically use a WaitGroup or channel to explicitly wait
	// for the goroutine to finish. Since the context is cancelled, the `runPipeline`
	// call should return quickly with an error.

	select {
	case res := <-resCh:
		// Check the error state
		if res.e == nil {
			t.Fatalf("Pipeline did not return an error after cancellation. Got result: %d", result)
		}

		// Check if the returned error is the expected context cancellation error
		if !errors.Is(res.e, context.Canceled) {
			t.Errorf("runPipeline returned the wrong error type after cancellation.\nGot: %v\nWant context.Canceled: %v", res.e, context.Canceled)
		}

	case <-timeoutCtx.Done():
		t.Fatalf("runPipeline goroutine did not return within the expected timeout after cancellation.")
	}

}
