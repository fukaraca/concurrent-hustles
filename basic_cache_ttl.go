// Question 9: Thread-Safe Cache with Expiration
// Implement a concurrent cache with TTL (time-to-live) support

package main

import (
	"fmt"
	"sync"
	"time"
)

type CacheItem struct {
	Value      any
	Expiration time.Time
}

type Cache struct {
	items    map[string]CacheItem
	mu       sync.RWMutex
	stop     chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// Create a new cache with background cleanup every cleanupInterval
func NewCache(cleanupInterval time.Duration) *Cache {
	c := &Cache{
		items: make(map[string]CacheItem),
		stop:  make(chan struct{}),
	}
	c.wg.Go(func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.cleanup()
			case <-c.stop:
				return
			}
		}
	})
	return c
}

// Add an item to the cache with a TTL
func (c *Cache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = CacheItem{
		Value:      value,
		Expiration: time.Now().Add(ttl),
	}
}

// Retrieve an item from the cache
// Return value and true if found and not expired
// Return nil and false if not found or expired
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.items[key]
	if !ok || v.Expiration.Before(time.Now()) {
		return nil, false
	}
	return v.Value, true
}

// Remove an item from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Background goroutine that removes expired items
func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.items {
		if v.Expiration.Before(now) {
			delete(c.items, k)
		}
	}
}

// Stop the cache cleanup goroutine and wait for it to finish
func (c *Cache) Close() {
	c.stopOnce.Do(func() {
		close(c.stop)
		c.wg.Wait()
	})
}

func main() {
	cache := NewCache(500 * time.Millisecond)
	defer cache.Close()

	// Test 1: Basic operations
	fmt.Println("=== Test 1: Basic operations ===")
	cache.Set("key1", "value1", 2*time.Second)
	if val, ok := cache.Get("key1"); ok {
		fmt.Printf("Got key1: %v\n", val)
	}

	// Test 2: Expiration
	fmt.Println("\n=== Test 2: Expiration ===")
	cache.Set("key2", "value2", 800*time.Millisecond)
	if val, ok := cache.Get("key2"); ok {
		fmt.Printf("Got key2 immediately: %v\n", val)
	}

	time.Sleep(1 * time.Second)
	if _, ok := cache.Get("key2"); !ok {
		fmt.Println("key2 expired (expected)")
	}

	// Test 3: Concurrent access
	fmt.Println("\n=== Test 3: Concurrent access ===")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", id)
			cache.Set(key, id, 2*time.Second)
			if val, ok := cache.Get(key); ok {
				fmt.Printf("Goroutine %d: got value %v\n", id, val)
			}
		}(i)
	}
	wg.Wait()

	time.Sleep(600 * time.Millisecond)
	fmt.Println("\nCache test completed")
}

// Test: Run with `go run main.go` and `go test -race`
// Expected: All operations should work correctly
// Expired items should not be retrievable
// Clean shutdown with Close()
