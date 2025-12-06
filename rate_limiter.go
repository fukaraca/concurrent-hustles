// Rate Limiter using Channels
// Implement a rate limiter that allows only N requests per second

package concurrent_hustles

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
