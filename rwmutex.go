package concurrent_hustles

import (
	"runtime"
	"sync/atomic"
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
