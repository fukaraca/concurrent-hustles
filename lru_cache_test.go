package concurrent_hustles

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestBasicGetPut tests basic Get and Put operations
func TestBasicGetPut(t *testing.T) {
	cache := NewLRUCache[string, int](3, 2)

	// Test Put and Get
	cache.Put("a", 1)
	if val, ok := cache.Get("a"); !ok || val != 1 {
		t.Errorf("Expected (1, true), got (%d, %v)", val, ok)
	}

	// Test Get non-existent key
	if _, ok := cache.Get("b"); ok {
		t.Error("Expected false for non-existent key")
	}
}

// TestLRUEviction tests that least recently used items are evicted
func TestLRUEviction(t *testing.T) {
	cache := NewLRUCache[string, int](3, 2)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	// Cache is full, adding "d" should evict "a" (least recently used)
	cache.Put("d", 4)

	if _, ok := cache.Get("a"); ok {
		t.Error("Expected 'a' to be evicted")
	}

	if val, ok := cache.Get("b"); !ok || val != 2 {
		t.Errorf("Expected (2, true), got (%d, %v)", val, ok)
	}
}

// TestUpdateExisting tests updating existing keys
func TestUpdateExisting(t *testing.T) {
	cache := NewLRUCache[string, int](3, 2)

	cache.Put("a", 1)
	cache.Put("a", 100)

	if val, ok := cache.Get("a"); !ok || val != 100 {
		t.Errorf("Expected (100, true), got (%d, %v)", val, ok)
	}
}

// TestGetPromotesItem tests that Get moves item to most recently used
func TestGetPromotesItem(t *testing.T) {
	cache := NewLRUCache[string, int](3, 2)

	cache.Put("a", 1)
	cache.Put("b", 2)
	cache.Put("c", 3)

	// Access "a" to make it most recently used
	cache.Get("a")

	// Add "d", should evict "b" (not "a")
	cache.Put("d", 4)

	if _, ok := cache.Get("b"); ok {
		t.Error("Expected 'b' to be evicted")
	}

	if val, ok := cache.Get("a"); !ok || val != 1 {
		t.Errorf("Expected 'a' to still exist with value 1, got (%d, %v)", val, ok)
	}
}

// TestConcurrentReads tests concurrent reads
func TestConcurrentReads(t *testing.T) {
	cache := NewLRUCache[int, string](100, 4)

	// Populate cache
	for i := 0; i < 50; i++ {
		cache.Put(i, fmt.Sprintf("value-%d", i))
	}

	var wg sync.WaitGroup
	numGoroutines := 50
	readsPerGoroutine := 1000

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < readsPerGoroutine; j++ {
				key := j % 50
				val, ok := cache.Get(key)
				if ok && val != fmt.Sprintf("value-%d", key) {
					t.Errorf("Goroutine %d: incorrect value for key %d", id, key)
				}
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrentWrites tests concurrent writes
func TestConcurrentWrites(t *testing.T) {
	cache := NewLRUCache[int, int](100, 4)

	var wg sync.WaitGroup
	numGoroutines := 50
	writesPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				key := id*writesPerGoroutine + j
				cache.Put(key, key*2)
			}
		}(i)
	}

	wg.Wait()

	// Verify some values (only the most recent 100 should exist due to capacity)
	count := 0
	for i := 0; i < numGoroutines*writesPerGoroutine; i++ {
		if _, ok := cache.Get(i); ok {
			count++
		}
	}

	if count > 100 {
		t.Errorf("Cache size exceeded capacity: %d > 100", count)
	}
}

// TestConcurrentReadWrite tests concurrent reads and writes
func TestConcurrentReadWrite(t *testing.T) {
	cache := NewLRUCache[string, int](50, 8)

	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	var writes, reads atomic.Int64

	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stopChan:
					return
				default:
					key := fmt.Sprintf("key-%d", rand.Intn(100))
					cache.Put(key, rand.Intn(1000))
					writes.Add(1)
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stopChan:
					return
				default:
					key := fmt.Sprintf("key-%d", rand.Intn(100))
					cache.Get(key)
					reads.Add(1)
				}
			}
		}(i)
	}

	// Run for a short duration
	time.Sleep(500 * time.Millisecond)
	close(stopChan)
	wg.Wait()

	t.Logf("Completed %d writes and %d reads", writes.Load(), reads.Load())
}

// TestConcurrentEviction tests that eviction works correctly under concurrent access
func TestConcurrentEviction(t *testing.T) {
	cache := NewLRUCache[int, int](10, 4)

	var wg sync.WaitGroup
	numGoroutines := 20

	// All goroutines write to same small cache, forcing evictions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := id*100 + j
				cache.Put(key, key)
			}
		}(i)
	}

	wg.Wait()

	// Count items in cache
	count := 0
	for i := 0; i < numGoroutines*100; i++ {
		if _, ok := cache.Get(i); ok {
			count++
		}
	}

	if count > 10 {
		t.Errorf("Cache size exceeded capacity after concurrent evictions: %d > 10", count)
	}
}

// TestConcurrentUpdateSameKey tests multiple goroutines updating the same key
func TestConcurrentUpdateSameKey(t *testing.T) {
	cache := NewLRUCache[string, int](10, 4)

	var wg sync.WaitGroup
	numGoroutines := 50
	key := "shared-key"

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cache.Put(key, id*1000+j)
			}
		}(i)
	}

	wg.Wait()

	// The key should exist with some value
	if _, ok := cache.Get(key); !ok {
		t.Error("Expected shared key to exist after concurrent updates")
	}
}

// TestRaceDetector tests for race conditions (run with -race flag)
func TestRaceDetector(t *testing.T) {
	cache := NewLRUCache[int, string](20, 4)

	var wg sync.WaitGroup

	// Mixed operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				cache.Put(j, fmt.Sprintf("val-%d-%d", id, j))
				cache.Get(j)
				cache.Put(j, fmt.Sprintf("updated-%d-%d", id, j))
			}
		}(i)
	}

	wg.Wait()
}

// TestIntTypes tests with integer keys and values
func TestIntTypes(t *testing.T) {
	cache := NewLRUCache[int, int](5, 2)

	for i := 0; i < 10; i++ {
		cache.Put(i, i*10)
	}

	// Only last 5 should exist
	for i := 0; i < 5; i++ {
		if _, ok := cache.Get(i); ok {
			t.Errorf("Key %d should have been evicted", i)
		}
	}

	for i := 5; i < 10; i++ {
		val, ok := cache.Get(i)
		if !ok || val != i*10 {
			t.Errorf("Key %d: expected (%d, true), got (%d, %v)", i, i*10, val, ok)
		}
	}
}

// BenchmarkConcurrentReads benchmarks concurrent read performance
func BenchmarkConcurrentReads(b *testing.B) {
	cache := NewLRUCache[int, int](1000, 16)

	for i := 0; i < 1000; i++ {
		cache.Put(i, i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Get(i % 1000)
			i++
		}
	})
}

// BenchmarkConcurrentWrites benchmarks concurrent write performance
func BenchmarkConcurrentWrites(b *testing.B) {
	cache := NewLRUCache[int, int](1000, 16)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Put(i%1000, i)
			i++
		}
	})
}

// BenchmarkConcurrentMixed benchmarks mixed read/write workload
func BenchmarkConcurrentMixed(b *testing.B) {
	cache := NewLRUCache[int, int](1000, 16)

	for i := 0; i < 500; i++ {
		cache.Put(i, i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%3 == 0 {
				cache.Put(i%1000, i)
			} else {
				cache.Get(i % 1000)
			}
			i++
		}
	})
}
