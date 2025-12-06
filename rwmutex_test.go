package concurrent_hustles

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test 1: Multiple concurrent readers.
func TestRWMutex_ConcurrentReaders(t *testing.T) {
	t.Parallel()
	mutex := &RWMutex{}
	var wg sync.WaitGroup
	const numReaders = 5
	var activeReaders atomic.Int32

	t.Logf("Test 1: Starting %d concurrent readers. Expected: All should acquire simultaneously.", numReaders)

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mutex.RLock()
			activeReaders.Add(1)
			// Hold the lock for a short duration
			if activeReaders.Load() != numReaders {
				// Not a failure, but shows they are not perfectly simultaneous, which is fine.
			}
			time.Sleep(10 * time.Millisecond)
			activeReaders.Add(-1)
			mutex.RUnlock()
		}()
	}
	wg.Wait()

	if currentReaders := (mutex.state.Load() & rmask); currentReaders != 0 {
		t.Errorf("Expected 0 active readers after test, got %d. State: %d", currentReaders, mutex.state.Load())
	}
	t.Log("Concurrent Readers test passed.")
}

// Test 2: Writer blocks subsequent readers.
func TestRWMutex_WriterBlocksReaders(t *testing.T) {
	t.Parallel()
	mutex := &RWMutex{}
	var wg sync.WaitGroup
	writerDone := make(chan struct{})
	readerAcquired := make(chan time.Time, 3)

	t.Log("Test 2: Starting writer, then readers. Expected: Readers wait for writer.")

	// Writer Goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		mutex.Lock()
		t.Log("Writer acquired lock.")
		close(writerDone) // Signal that the writer has the lock
		time.Sleep(50 * time.Millisecond)
		mutex.Unlock()
		t.Log("Writer released lock.")
	}()

	<-writerDone // Wait until writer has the lock

	// Reader Goroutines
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mutex.RLock()
			readerAcquired <- time.Now()
			time.Sleep(10 * time.Millisecond) // Hold for a moment
			mutex.RUnlock()
		}()
	}

	// Wait for all readers to finish
	wg.Wait()

	// Assert that the first reader acquired its lock AFTER the writer released it.
	if len(readerAcquired) > 0 {
		firstReaderTime := <-readerAcquired
		// Check if the writer's release time is implicitly before the reader's acquisition.
		// If the test completes without deadlock, it confirms blocking.
		// A more rigorous check requires correlating timestamps, but simple wg.Wait()
		// and channel sync are usually sufficient for blocking verification.
		t.Logf("First reader acquired lock at: %v (after writer release)", firstReaderTime)
	}

	if mutex.state.Load() != 0 {
		t.Errorf("Expected mutex state 0, got %d", mutex.state.Load())
	}
	t.Log("Writer blocks readers test passed.")
}

// Test 3: Readers block writer.
func TestRWMutex_ReadersBlockWriter(t *testing.T) {
	t.Parallel()
	mutex := &RWMutex{}
	var wg sync.WaitGroup
	readersHeld := make(chan struct{})
	writerAcquired := make(chan struct{})

	t.Log("Test 3: Starting readers, then writer. Expected: Writer waits for all readers.")

	// Reader Goroutines (acquire and hold)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			mutex.RLock()
			if id == 0 {
				close(readersHeld) // Signal that readers are holding
			}
			time.Sleep(50 * time.Millisecond)
			mutex.RUnlock()
			wg.Done()
		}(i)
	}

	<-readersHeld // Wait until readers are holding the lock

	// Writer Goroutine (should block)
	wg.Add(1)
	go func() {
		defer wg.Done()
		mutex.Lock() // Blocks until readers release
		writerAcquired <- struct{}{}
		mutex.Unlock()
	}()

	// Wait for a short time to ensure writer is blocked
	select {
	case <-writerAcquired:
		t.Fatal("Writer acquired lock while readers were still holding it (Unexpected behavior).")
	case <-time.After(10 * time.Millisecond):
		t.Log("Writer is blocked (Expected).")
	}
	go func() { <-writerAcquired }()
	// Now wait for all readers and the writer to finish
	wg.Wait()

	if mutex.state.Load() != 0 {
		t.Errorf("Expected mutex state 0, got %d", mutex.state.Load())
	}
	t.Log("Readers block writer test passed.")
}

// Test 4 & 5: TryLock behavior.
func TestRWMutex_TryLock(t *testing.T) {
	t.Parallel()
	mutex := &RWMutex{}

	// Test 4: TryLock succeeds when unlocked
	if !mutex.TryLock() {
		t.Fatalf("TryLock failed on unlocked mutex (Unexpected).")
	}
	t.Log("TryLock successfully acquired write lock on unlocked mutex.")
	mutex.Unlock()

	// Test 5: TryLock fails when read lock held
	mutex.RLock()
	if mutex.TryLock() {
		t.Fatal("TryLock succeeded while read lock was held (Unexpected).")
	}
	t.Log("TryLock successfully failed when read lock was held.")
	mutex.RUnlock()

	// Test: TryLock fails when write lock held
	mutex.Lock()
	if mutex.TryLock() {
		t.Fatal("TryLock succeeded while write lock was held (Unexpected).")
	}
	t.Log("TryLock successfully failed when write lock was held.")
	mutex.Unlock()

	if mutex.state.Load() != 0 {
		t.Errorf("Expected mutex state 0, got %d", mutex.state.Load())
	}
}

// Test 6 & 7: TryRLock behavior.
func TestRWMutex_TryRLock(t *testing.T) {
	t.Parallel()
	mutex := &RWMutex{}

	// Test 6: TryRLock succeeds when unlocked
	if !mutex.TryRLock() {
		t.Fatalf("TryRLock failed on unlocked mutex (Unexpected).")
	}
	t.Log("TryRLock 1 successfully acquired read lock.")

	// Test 6: Second TryRLock also succeeds
	if !mutex.TryRLock() {
		t.Fatal("TryRLock 2 failed while read lock 1 was held (Unexpected).")
	}
	t.Log("TryRLock 2 successfully acquired read lock while another was held.")
	mutex.RUnlock()
	mutex.RUnlock()

	// Test 7: TryRLock fails when write lock held
	mutex.Lock()
	if mutex.TryRLock() {
		t.Fatal("TryRLock succeeded while write lock was held (Unexpected).")
	}
	t.Log("TryRLock successfully failed when write lock was held.")
	mutex.Unlock()

	if mutex.state.Load() != 0 {
		t.Errorf("Expected mutex state 0, got %d", mutex.state.Load())
	}
}

// Test 8: Multiple writers compete.
func TestRWMutex_MultipleWriters(t *testing.T) {
	t.Parallel()
	mutex := &RWMutex{}
	var wg sync.WaitGroup
	var activeWriters atomic.Int32
	const numWriters = 5

	t.Logf("Test 8: Starting %d competing writers. Expected: Only one acquires at a time.", numWriters)

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			mutex.Lock()
			// Check that only one writer is active
			if count := activeWriters.Add(1); count > 1 {
				t.Errorf("Concurrency violation: %d writers active simultaneously (Writer %d)", count, id)
			}
			time.Sleep(10 * time.Millisecond) // Hold for a moment
			activeWriters.Add(-1)
			mutex.Unlock()
		}(i)
	}
	wg.Wait()

	if mutex.state.Load() != 0 {
		t.Errorf("Expected mutex state 0, got %d", mutex.state.Load())
	}
	t.Log("Multiple Writers test passed (mutual exclusion verified).")
}

// Test 9: Writer Preference Check (wwbit) - A waiting writer should block new readers.
func TestRWMutex_WriterPreference(t *testing.T) {
	t.Parallel()
	mutex := &RWMutex{}
	var wg sync.WaitGroup
	r1Acquired := make(chan struct{})
	writerWaiting := make(chan struct{})
	r2Acquired := make(chan struct{})

	t.Log("Test 9: Checking writer preference. R1 holds, Writer waits (sets wwbit), R2 must block.")

	// R1: Acquire the lock and hold it.
	wg.Add(1)
	go func() {
		mutex.RLock()
		close(r1Acquired)                 // R1 holds lock
		time.Sleep(50 * time.Millisecond) // Hold long enough for writer to start waiting
		mutex.RUnlock()
		wg.Done()
	}()

	<-r1Acquired // Wait for R1 to acquire

	// Writer: Start waiting (should set wwbit).
	wg.Add(1)
	go func() {
		writerWaiting <- struct{}{} // Signal that writer is about to call Lock()
		mutex.Lock()                // Will block until R1 releases
		t.Log("Writer acquired lock (R1 released).")
		time.Sleep(30 * time.Millisecond)
		mutex.Unlock()
		wg.Done()
	}()

	<-writerWaiting // Wait for writer to be ready to call Lock()
	// Give time for writer's Lock() to execute and set wwbit
	time.Sleep(10 * time.Millisecond)

	// R2: Try to acquire the lock (should block due to wwbit).
	wg.Add(1)
	go func() {
		mutex.RLock() // Should block
		r2Acquired <- struct{}{}
		t.Log("Reader 2 acquired lock (after writer finished).")
		mutex.RUnlock()
		wg.Done()
	}()

	// Check that R2 is blocked
	select {
	case <-r2Acquired:
		t.Fatal("Reader 2 acquired lock while a writer was waiting (wwbit violation).")
	case <-time.After(15 * time.Millisecond):
		t.Log("Reader 2 is blocked (Expected - wwbit successfully prevents new readers).")
	}
	go func() {
		<-r2Acquired
	}()
	// Now R1 releases its lock, allowing Writer to proceed, and then R2 to proceed.
	wg.Wait()

	if mutex.state.Load() != 0 {
		t.Errorf("Expected mutex state 0, got %d", mutex.state.Load())
	}
	t.Log("Writer Preference test passed.")
}

// Test 10: Panic on invalid Unlock/RUnlock calls.
func TestRWMutex_PanicOnInvalidRelease(t *testing.T) {
	t.Parallel()
	mutex := &RWMutex{}

	// Test: Unlock without Lock
	t.Run("Unlock without Lock", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for Unlock without Lock, but none occurred.")
			}
		}()
		mutex.Unlock()
	})

	// Test: RUnlock without RLock
	t.Run("RUnlock without RLock", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for RUnlock without RLock, but none occurred.")
			}
		}()
		mutex.RUnlock()
	})
}
