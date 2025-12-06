package concurrent_hustles

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRateLimiterTiming verifies that the rate limiter correctly spaces out
// requests after the initial burst capacity is used.
func TestRateLimiterTiming(t *testing.T) {
	const maxRate = 2       // 2 requests per second (500ms interval)
	const totalRequests = 6 // Use 6 total requests
	const expectedWaitInterval = 500 * time.Millisecond
	const burstCapacity = maxRate

	// Initial burst (2 requests) are instantaneous.
	// Remaining 4 requests must wait for new tokens.
	// Total minimum wait time = (Total Requests - Burst Capacity) * Interval
	// Min time = (6 - 2) * 500ms -200 = 1800ms 200ms is added since start ticker initilized 50ms before
	minDuration := time.Duration(totalRequests-burstCapacity)*expectedWaitInterval - 200*time.Millisecond
	// Add a small buffer for safety due to goroutine scheduling
	maxDuration := minDuration + 200*time.Millisecond

	limiter := NewRateLimiter(maxRate)
	go limiter.Start()
	// Ensure the Start goroutine has time to initialize the Ticker
	time.Sleep(50 * time.Millisecond)
	defer limiter.Stop()

	var wg sync.WaitGroup
	start := time.Now()
	var allowedCount atomic.Int32

	// Try to make 6 requests
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if limiter.Allow() {
				// Atomically increment the counter if allowed
				allowedCount.Add(1)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// 1. Check if the correct number of requests were processed
	if allowedCount.Load() != totalRequests {
		t.Errorf("Expected %d requests to be allowed, got %d", totalRequests, allowedCount)
	}

	// 2. Check the timing of the requests
	if elapsed < minDuration || elapsed > maxDuration {
		t.Errorf("Rate limiting duration failed. Expected between %v and %v, got %v", minDuration, maxDuration, elapsed)
	}
}

// TestRateLimiterStop verifies that Allow() returns false after Stop() is called.
func TestRateLimiterStop(t *testing.T) {
	limiter := NewRateLimiter(5)
	go limiter.Start()
	time.Sleep(50 * time.Millisecond) // Give Start a moment to initialize the ticker

	// 1. Consume the initial burst capacity (5 tokens)
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Fatalf("Allow() failed unexpectedly before stop was called")
		}
	}

	// 2. Stop the rate limiter
	limiter.Stop()
	// Give the Start loop a moment to detect IsStopped() and close the tokens channel
	time.Sleep(100 * time.Millisecond)

	// 3. Try to get a new token - this should fail immediately
	if limiter.Allow() {
		t.Errorf("Allow() succeeded after Stop() was called, expected false")
	}

	// 4. Check if IsStopped is true
	if !limiter.IsStopped() {
		t.Errorf("IsStopped() returned false after Stop() was called, expected true")
	}
}
