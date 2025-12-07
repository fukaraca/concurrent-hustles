package concurrent_hustles

import (
	"hash/maphash"
	"sync"
	"weak"
)

type LRUCache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Put(key K, value V)
}

var _ LRUCache[string, int] = (*lruCache[string, int])(nil)

type lruCache[K comparable, V any] struct {
	seed       maphash.Seed
	mu         sync.Mutex
	capacity   int
	len        int
	shards     uint64
	head, tail *cacheNode[K, V]
	cache      []*shardOfMap[K, V]
}

type cacheNode[K comparable, V any] struct {
	key       K
	val       V
	next, pre *cacheNode[K, V]
}

type shardOfMap[K comparable, V any] struct {
	id int
	m  map[K]weak.Pointer[cacheNode[K, V]]
}

func (c *shardOfMap[K, V]) get(key K) (*cacheNode[K, V], bool) {
	v, ok := c.m[key]
	if !ok {
		return nil, false
	}
	if ptr := v.Value(); ptr != nil {
		return ptr, true
	}
	c.delete(key)
	return nil, false
}

func (c *shardOfMap[K, V]) put(key K, node *cacheNode[K, V]) {
	c.m[key] = weak.Make(node)
}

func (c *shardOfMap[K, V]) delete(key K) {
	delete(c.m, key)
}

func NewLRUCache[K comparable, V any](capacity, shards int) LRUCache[K, V] {
	var head, tail cacheNode[K, V]
	head.next = &tail
	tail.pre = &head
	maps := make([]*shardOfMap[K, V], shards)
	for i := range shards {
		maps[i] = &shardOfMap[K, V]{
			id: i,
			m:  make(map[K]weak.Pointer[cacheNode[K, V]]),
		}
	}
	return &lruCache[K, V]{
		capacity: capacity,
		shards:   uint64(shards),
		head:     &head,
		tail:     &tail,
		cache:    maps,
		seed:     maphash.MakeSeed(),
	}
}

func (c *lruCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.findInCache(key)
	if !ok {
		var zero V
		return zero, false
	}
	c.moveToTail(v)
	return v.val, true
}

func (c *lruCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.findInCache(key)
	if ok {
		v.val = value
		c.moveToTail(v)
		return
	}
	if c.capacity == c.len {
		c.evictLRU()
	}
	node := cacheNode[K, V]{key: key, val: value}
	c.addToTail(&node)
	c.addToMap(key, &node)
	c.len++
}

func (c *lruCache[K, V]) findShard(key K) int {
	return int(maphash.Comparable(c.seed, key) % c.shards)
}

func (c *lruCache[K, V]) deleteFromMap(key K) {
	c.cache[c.findShard(key)].delete(key)
}

func (c *lruCache[K, V]) addToMap(key K, node *cacheNode[K, V]) {
	c.cache[c.findShard(key)].put(key, node)
}

func (c *lruCache[K, V]) findInCache(key K) (*cacheNode[K, V], bool) {
	return c.cache[c.findShard(key)].get(key)
}

// promote to LRU
func (c *lruCache[K, V]) moveToTail(node *cacheNode[K, V]) {
	c.remove(node)
	c.addToTail(node)
}

// newly added
func (c *lruCache[K, V]) addToTail(node *cacheNode[K, V]) {
	node.next, node.pre = c.tail, c.tail.pre
	c.tail.pre.next, c.tail.pre = node, node
}

// unlink the nodes
func (c *lruCache[K, V]) remove(node *cacheNode[K, V]) {
	node.next.pre, node.pre.next = node.pre, node.next
}

// both unlink and remove from cache
func (c *lruCache[K, V]) evictLRU() {
	if c.len == 0 {
		return
	}
	lru := c.head.next
	c.head.next = lru.next
	lru.next.pre = c.head
	c.deleteFromMap(lru.key)
	c.len--
}
