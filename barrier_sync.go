//Barrier Synchronization
// Implement a barrier that synchronizes multiple goroutines at a point

package concurrent_hustles

import (
	"fmt"
	"sync"
	"time"
)

type Barrier struct {
	count      int
	waiting    int
	mu         sync.Mutex
	cond       *sync.Cond
	generation int
}

// Create a barrier for n goroutines
func NewBarrier(n int) *Barrier {
	b := &Barrier{
		count: n,
	}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Block until all goroutines reach the barrier
// When all goroutines arrive, release them all at once
// Must handle multiple rounds (generations) correctly
func (b *Barrier) Wait() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.waiting++
	gen := b.generation
	if b.waiting == b.count {
		b.waiting = 0
		b.generation++
		b.cond.Broadcast()
		return
	}
	for gen == b.generation {
		b.cond.Wait() // we can awake early, so we check again if gen incremented by last caller
	}

}

func worker(id int, barrier *Barrier, phases int) {
	for phase := 0; phase < phases; phase++ {
		// Simulate work
		workTime := time.Duration(100+id*50) * time.Millisecond
		time.Sleep(workTime)

		fmt.Printf("Worker %d completed phase %d\n", id, phase)

		// Wait for all workers to complete this phase
		barrier.Wait()

		fmt.Printf("Worker %d proceeding to phase %d\n", id, phase+1)
	}
	fmt.Printf("Worker %d finished all phases\n", id)
}
