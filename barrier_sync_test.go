package concurrent_hustles

import (
	"sync"
	"testing"
	"time"
)

func TestBarrierWorkersRaceFree(t *testing.T) {
	numWorkers := 5
	numPhases := 10

	barrier := NewBarrier(numWorkers)
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			worker(id, barrier, numPhases)
		}(i)
	}

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	// Avoid hanging tests forever if something is wrong
	select {
	case <-done:
		elapsed := time.Since(start)
		t.Logf("All workers completed in %v", elapsed)
	case <-time.After(10 * time.Second):
		t.Fatalf("workers did not complete within timeout (possible deadlock)")
	}
}

func TestBarrierMultipleGenerations(t *testing.T) {
	numWorkers := 4
	numPhases := 20

	barrier := NewBarrier(numWorkers)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for phase := 0; phase < numPhases; phase++ {
				// some tiny fake work
				time.Sleep(time.Millisecond * 5)
				barrier.Wait()
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatalf("barrier did not allow all %d phases to complete", numPhases)
	}
}
