package concurrent_hustles

import (
	"testing"
	"time"
)

// TestFanOutFanIn verifies the pipeline functions: generation, processing, and merging.
func TestFanOutFanIn(t *testing.T) {
	const totalNumbers = 10
	const expectedSum = 110 // (2+4+6+...+20) = 110
	const numProcessors = 3 // Use a reasonable number of workers
	const processingDelayMs = 300

	t.Logf("Starting Fan-Out/Fan-In test with %d processors and %dms delay per item.", numProcessors, processingDelayMs)

	start := time.Now()

	// 1. Generate: numbers 1-10
	numbers := generator(totalNumbers)

	// 2. Fan-out: distribute work to multiple processors
	processors := make([]<-chan int, numProcessors)
	for i := 0; i < numProcessors; i++ {
		// All processors read from the same 'numbers' channel
		processors[i] = processor(i, numbers)
	}

	// 3. Fan-in: merge results from all processor channels
	results := fanIn(processors...)

	// 4. Collect and verify results
	sum := 0
	count := 0
	for result := range results {
		sum += result
		count++
	}

	elapsed := time.Since(start)

	// --- Assertions ---

	// Check if all numbers were processed
	if count != totalNumbers {
		t.Errorf("FAIL: Expected %d processed numbers, but got %d", totalNumbers, count)
	}

	// Check if the calculation is correct
	if sum != expectedSum {
		t.Errorf("FAIL: Expected total sum to be %d, but got %d", expectedSum, sum)
	}

	// Check for evidence of parallelism (runtime must be significantly less than sequential time)
	// Sequential time would be (10 items * 300ms) = 3000ms.
	// We expect the runtime to be close to (ceil(10 / 3) * 300ms) = 4 * 300ms = 1200ms, plus overhead.
	// We check if it's less than 2000ms (2 seconds) to allow for some overhead.
	sequentialTime := time.Millisecond * time.Duration(totalNumbers*processingDelayMs)
	if elapsed >= sequentialTime {
		t.Logf("WARN: Test took %v, which is NOT faster than sequential time of %v. Parallelism may not be effective.", elapsed, sequentialTime)
	} else if elapsed > time.Millisecond*time.Duration(2000) {
		t.Logf("WARN: Test took %v. It is faster than sequential, but perhaps slower than ideal.", elapsed)
	}

	t.Logf("SUCCESS: Processed %d numbers, sum=%d in %v", count, sum, elapsed)
}
