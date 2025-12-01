// Question 2: Rate Limiter using Channels
// Implement a rate limiter that allows only N requests per second

package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type RateLimiter struct {
	ticker  *time.Ticker
	tokens  chan struct{}
	maxRate int
	mu      sync.Mutex
	stopped atomic.Bool
}

// Create a rate limiter that allows maxRate requests per second
func NewRateLimiter(maxRate int) *RateLimiter {
	r := &RateLimiter{
		ticker:  new(time.Ticker),
		tokens:  make(chan struct{}, maxRate),
		maxRate: maxRate,
		stopped: atomic.Bool{},
	}

	for range maxRate {
		r.tokens <- struct{}{}
	}
	return r
}

// Start the rate limiter token generation
func (rl *RateLimiter) Start() {
	rl.ticker = time.NewTicker(time.Second / time.Duration(rl.maxRate))
	defer rl.ticker.Stop()
	for {
		<-rl.ticker.C
		if rl.IsStopped() {
			close(rl.tokens)
			fmt.Printf("closing rl \n")
			return
		}
		select {
		case rl.tokens <- struct{}{}:
		default:

		}

	}
}

// Block until a token is available, then return true
// Return false if rate limiter is stopped
func (rl *RateLimiter) Allow() bool {
	if rl.IsStopped() {
		return false
	}
	<-rl.tokens
	return true
}

// Stop the rate limiter
func (rl *RateLimiter) Stop() {
	rl.stopped.CompareAndSwap(false, true)
}

func (rl *RateLimiter) IsStopped() bool {
	return rl.stopped.Load()
}

func main() {
	limiter := NewRateLimiter(2) // 2 requests per second
	go limiter.Start()
	defer limiter.Stop()

	start := time.Now()
	var wg sync.WaitGroup

	// Try to make 6 requests
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if limiter.Allow() {
				elapsed := time.Since(start).Seconds()
				fmt.Printf("Request %d allowed at %.2fs\n", id, elapsed)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start).Seconds()
	fmt.Printf("Total time: %.2fs (should be ~3s for 6 requests at 2/sec)\n", elapsed)
}

// Test: Run with `go run main.go` and `go test -race`
// Expected: 6 requests should take approximately 3 seconds
// Requests should be evenly spaced at ~0.5s intervals
