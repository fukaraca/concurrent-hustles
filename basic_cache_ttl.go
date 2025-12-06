// Thread-Safe Cache with Expiration
// Implement a concurrent cache with TTL (time-to-live) support

package concurrent_hustles

import (
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
