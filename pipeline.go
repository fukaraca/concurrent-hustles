// Question 7: Pipeline Pattern with Error Handling
// Implement a cancellable pipeline that processes data through multiple stages

package main

import (
	"context"
	"fmt"
	"sync"
)

// Generate numbers from 1 to n
// Must respect context cancellation
func stage1(ctx context.Context, n int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 1; i <= n; i++ {
			select {
			case <-ctx.Done():
				return
			case ch <- i:
				fmt.Printf("stage %d initiated \n", 1)
			}
		}
	}()
	return ch
}

// Filter out even numbers
// Must respect context cancellation
func stage2(ctx context.Context, input <-chan int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case i, ok := <-input:
				fmt.Printf("stage %d initiated %v \n", 2, ok)
				if !ok {
					return
				}
				if i%2 == 0 {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case ch <- i:
				}
			}
		}
	}()
	return ch
}

// Square the numbers
// Must respect context cancellation
func stage3(ctx context.Context, input <-chan int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case i, ok := <-input:
				fmt.Printf("stage %d initiated %v \n", 3, ok)
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case ch <- i * i:
				}
			}
		}
	}()
	return ch
}

// Sum all numbers and send final result
// Must respect context cancellation
// Return a channel that will receive the sum
func stage4(ctx context.Context, input <-chan int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		var out int
		for {
			select {
			case <-ctx.Done():
				return
			case i, ok := <-input:
				fmt.Printf("stage %d initiated %v \n", 4, ok)
				if !ok {
					select {
					case <-ctx.Done():
					case ch <- out:
					}
					return
				}
				out += i
			}
		}
	}()
	return ch
}

// Connect all stages and return the final result
// Handle cancellation properly - all stages should stop when context is cancelled
func runPipeline(ctx context.Context, n int) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	r, ok := <-stage4(ctx, stage3(ctx, stage2(ctx, stage1(ctx, n))))
	if !ok {
		return 0, fmt.Errorf("pipeline failed as %w", ctx.Err())
	}

	return r, nil
}

func main() {
	fmt.Println("=== Test 1: Complete pipeline ===")
	ctx1 := context.Background()
	result1, err1 := runPipeline(ctx1, 10)
	if err1 != nil {
		fmt.Printf("Error: %v\n", err1)
	} else {
		fmt.Printf("Result: %d\n", result1)
		fmt.Printf("Expected: %d (1² + 3² + 5² + 7² + 9² = 1+9+25+49+81 = 165)\n", 165)
	}

	fmt.Println("\n=== Test 2: Cancelled pipeline ===")
	ctx2, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result2, err2 := runPipeline(ctx2, 1000000)
		if err2 != nil {
			fmt.Printf("Pipeline cancelled (expected): %v\n", err2)
		} else {
			fmt.Printf("Result: %d\n", result2)
		}
	}()

	// Cancel after a short delay
	cancel()
	wg.Wait()
	fmt.Println("Pipeline shutdown complete")
}

// Test: Run with `go run main.go` and `go test -race`
// Expected Test 1: Result should be 165
// Expected Test 2: Pipeline should cancel gracefully
// No goroutine leaks, no race conditions
