package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type RWMutex struct {
	state atomic.Int64
}

const wbit int64 = 1 << 62
const wwbit int64 = 1 << 61
const rmask = ^(wbit | wwbit)

// Lock acquires a write lock.
// Expected behavior:
// - Blocks if another goroutine holds a write lock (Lock)
// - Blocks if one or more goroutines hold read locks (RLock)
// - Only one writer can hold the lock at a time
// - Prevents any new readers from acquiring the lock while waiting
func (m *RWMutex) Lock() {
	for {
		s := m.state.Load()
		if s&(wbit|rmask) != 0 {
			m.state.CompareAndSwap(s, s|wwbit)
			runtime.Gosched()
			continue
		}
		if m.state.CompareAndSwap(s, wbit) {
			return
		}
		runtime.Gosched()
	}
}

// Unlock releases a write lock.
// Expected behavior:
// - Should only be called after a successful Lock()
// - Allows waiting readers or writers to proceed
// - Calling without holding the lock is undefined behavior
func (m *RWMutex) Unlock() {
	for {
		s := m.state.Load()
		if s&wbit == 0 {
			panic("cannot unlock unlocked mutex")
		}
		if m.state.CompareAndSwap(s, 0) {
			return
		}
		runtime.Gosched()
	}
}

// RLock acquires a read lock.
// Expected behavior:
// - Blocks if a goroutine holds a write lock (Lock)
// - Allows multiple readers to hold the lock simultaneously
// - Does not block other RLock calls
// - May block if there are waiting writers (writer-preferring) or may proceed (reader-preferring)
func (m *RWMutex) RLock() {
	for {
		s := m.state.Load()
		if s&(wbit|wwbit) != 0 {
			runtime.Gosched()
			continue
		}
		if m.state.CompareAndSwap(s, s+1) {
			return
		}
		runtime.Gosched()
	}
}

// RUnlock releases a read lock.
// Expected behavior:
// - Should only be called after a successful RLock()
// - Decrements the reader count
// - If this is the last reader, allows waiting writers to proceed
// - Calling without holding a read lock is undefined behavior
func (m *RWMutex) RUnlock() {
	for {
		s := m.state.Load()
		if s&rmask == 0 {
			panic("cannot runlock runlocked mutex")

		}
		if m.state.CompareAndSwap(s, s-1) {
			return
		}
		runtime.Gosched()
	}
}

// TryLock attempts to acquire a write lock without blocking.
// Expected behavior:
// - Returns true if lock was acquired successfully
// - Returns false immediately if lock is held by any reader or writer
// - Does not block/wait
func (m *RWMutex) TryLock() bool {
	if s := m.state.Load(); s == 0 {
		return m.state.CompareAndSwap(s, wbit)
	}
	return false
}

// TryRLock attempts to acquire a read lock without blocking.
// Expected behavior:
// - Returns true if read lock was acquired successfully
// - Returns false immediately if lock is held by a writer
// - May return false if writers are waiting (depending on fairness policy)
// - Does not block/wait
func (m *RWMutex) TryRLock() bool {
	if s := m.state.Load(); s&(wbit|wwbit) == 0 {
		return m.state.CompareAndSwap(s, s+1)
	}
	return false
}

func main() {
	mutex := &RWMutex{}
	var wg sync.WaitGroup

	fmt.Println("=== Test 1: Multiple concurrent readers ===")
	// Expected: All readers should be able to acquire lock simultaneously
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			mutex.RLock()
			fmt.Printf("Reader %d: acquired read lock\n", id)
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("Reader %d: releasing read lock\n", id)
			mutex.RUnlock()
		}(i)
	}
	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	fmt.Println("\n=== Test 2: Writer blocks readers ===")
	// Expected: Writer acquires lock, readers wait, then readers proceed after writer releases
	wg.Add(1)
	go func() {
		defer wg.Done()
		mutex.Lock()
		fmt.Println("Writer: acquired write lock")
		time.Sleep(300 * time.Millisecond)
		fmt.Println("Writer: releasing write lock")
		mutex.Unlock()
	}()

	time.Sleep(50 * time.Millisecond) // Ensure writer gets lock first

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			fmt.Printf("Reader %d: waiting for read lock\n", id)
			mutex.RLock()
			fmt.Printf("Reader %d: acquired read lock\n", id)
			time.Sleep(50 * time.Millisecond)
			mutex.RUnlock()
		}(i)
	}
	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	fmt.Println("\n=== Test 3: Readers block writer ===")
	// Expected: Readers acquire locks, writer waits, then writer proceeds after all readers release
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			mutex.RLock()
			fmt.Printf("Reader %d: acquired read lock\n", id)
			time.Sleep(300 * time.Millisecond)
			fmt.Printf("Reader %d: releasing read lock\n", id)
			mutex.RUnlock()
		}(i)
	}

	time.Sleep(50 * time.Millisecond) // Ensure readers get locks first

	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("Writer: waiting for write lock")
		mutex.Lock()
		fmt.Println("Writer: acquired write lock")
		time.Sleep(100 * time.Millisecond)
		mutex.Unlock()
	}()
	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	fmt.Println("\n=== Test 4: TryLock - successful acquisition ===")
	// Expected: TryLock succeeds when no lock is held
	if mutex.TryLock() {
		fmt.Println("TryLock: successfully acquired write lock")
		mutex.Unlock()
	} else {
		fmt.Println("TryLock: failed to acquire write lock")
	}
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n=== Test 5: TryLock - fails when read lock held ===")
	// Expected: TryLock fails when readers hold the lock
	mutex.RLock()
	fmt.Println("Reader: acquired read lock")

	if mutex.TryLock() {
		fmt.Println("TryLock: successfully acquired write lock (unexpected!)")
		mutex.Unlock()
	} else {
		fmt.Println("TryLock: failed to acquire write lock (expected)")
	}

	mutex.RUnlock()
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n=== Test 6: TryRLock - successful acquisition ===")
	// Expected: Multiple TryRLock succeed simultaneously
	if mutex.TryRLock() {
		fmt.Println("TryRLock 1: successfully acquired read lock")
		if mutex.TryRLock() {
			fmt.Println("TryRLock 2: successfully acquired read lock")
			mutex.RUnlock()
		}
		mutex.RUnlock()
	}
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n=== Test 7: TryRLock - fails when write lock held ===")
	// Expected: TryRLock fails when a writer holds the lock
	mutex.Lock()
	fmt.Println("Writer: acquired write lock")

	if mutex.TryRLock() {
		fmt.Println("TryRLock: successfully acquired read lock (unexpected!)")
		mutex.RUnlock()
	} else {
		fmt.Println("TryRLock: failed to acquire read lock (expected)")
	}

	mutex.Unlock()
	time.Sleep(100 * time.Millisecond)

	fmt.Println("\n=== Test 8: Multiple writers compete ===")
	// Expected: Only one writer can hold the lock at a time
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			mutex.Lock()
			fmt.Printf("Writer %d: acquired write lock\n", id)
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("Writer %d: releasing write lock\n", id)
			mutex.Unlock()
		}(i)
	}
	wg.Wait()

	fmt.Println("\n=== All tests completed ===")
}
