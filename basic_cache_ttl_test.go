package concurrent_hustles

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestCacheBasicAndExpiration(t *testing.T) {
	t.Helper()

	cleanupInterval := 20 * time.Millisecond
	cache := NewCache(cleanupInterval)
	defer cache.Close()

	// Set a key with short TTL
	ttl := 50 * time.Millisecond
	cache.Set("foo", "bar", ttl)

	// Immediately retrievable
	if v, ok := cache.Get("foo"); !ok {
		t.Fatalf("expected key foo to be present")
	} else if v.(string) != "bar" {
		t.Fatalf("expected value 'bar', got %v", v)
	}

	// Wait for it to expire
	time.Sleep(ttl + 10*time.Millisecond)

	// Give cleanup a bit of time to run
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		if _, ok := cache.Get("foo"); !ok {
			// Either expired in Get check or removed by cleanup.
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected key foo to be expired and not retrievable")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCacheConcurrentAccessAndClose(t *testing.T) {
	t.Helper()

	cache := NewCache(10 * time.Millisecond)

	var wg sync.WaitGroup
	numWorkers := 16
	stopWorkers := make(chan struct{})

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "k" + strconv.Itoa(id)

			for {
				select {
				case <-stopWorkers:
					return
				default:
					// Exercise Set/Get/Delete under concurrent usage
					cache.Set(key, id, 200*time.Millisecond)
					cache.Get(key) // ignore result; we just care about race safety
					cache.Delete(key)
				}
			}
		}(i)
	}

	// Let them hammer the cache for a short while
	time.Sleep(200 * time.Millisecond)
	close(stopWorkers)
	wg.Wait()

	// Now ensure Close returns and doesn't deadlock
	done := make(chan struct{})
	go func() {
		cache.Close()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatalf("Close did not return in time (possible deadlock)")
	}
}

// Ensure Close is safe to call from multiple goroutines (idempotent via stopOnce).
func TestCacheCloseIdempotent(t *testing.T) {
	t.Helper()

	cache := NewCache(30 * time.Millisecond)

	var wg sync.WaitGroup
	numClosers := 10

	for i := 0; i < numClosers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Close()
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// good, all Close calls returned
	case <-time.After(2 * time.Second):
		t.Fatalf("concurrent Close calls did not finish in time")
	}
}
