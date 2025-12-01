// Question 3: Fan-Out, Fan-In Pattern
// Implement a pipeline that fans out work to multiple processors and fans in results

package main

import (
	"fmt"
	"sync"
	"time"
)

// Generate numbers from 1 to n and send them to the returned channel
func generator(n int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 1; i <= n; i++ {
			ch <- i
		}
	}()
	return ch
}

// Process numbers from input channel (multiply by 2)
// Send results to output channel
// Each processor should add a small delay to simulate work
func processor(id int, input <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for c := range input {
			time.Sleep(time.Millisecond * time.Duration(300))
			fmt.Printf("processor %d proc %d\n", id, c)
			out <- c * 2
		}
	}()
	return out
}

// Merge multiple input channels into a single output channel
// Must handle dynamic closing of input channels properly
func fanIn(channels ...<-chan int) <-chan int {
	out := make(chan int)
	go func() {
		var wg sync.WaitGroup
		for _, channel := range channels {
			ch := channel
			wg.Go(func() {
				for c := range ch {
					out <- c
				}
			})
		}
		wg.Wait()
		close(out)
	}()

	return out
}

func main() {
	start := time.Now()

	// Generate numbers 1-10
	numbers := generator(10)

	// Fan-out to 3 processors
	numProcessors := 20
	processors := make([]<-chan int, numProcessors)
	for i := 0; i < numProcessors; i++ {
		processors[i] = processor(i, numbers)
	}

	// Fan-in results
	results := fanIn(processors...)

	// Collect and display results
	sum := 0
	count := 0
	for result := range results {
		fmt.Printf("Received: %d\n", result)
		sum += result
		count++
	}

	elapsed := time.Since(start)
	fmt.Printf("\nProcessed %d numbers, sum=%d in %v\n", count, sum, elapsed)
	fmt.Printf("Expected sum: %d (2+4+6+...+20 = 110)\n", 110)
}

// Test: Run with `go run main.go` and `go test -race`
// Expected: All 10 numbers processed, sum should be 110
// Should take less time than sequential processing due to parallelism
// No race conditions or deadlocks
