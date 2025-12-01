// Question 10: Bounded Queue with Blocking Operations
// Implement a thread-safe bounded queue with blocking enqueue/dequeue

package main

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrQueueFull   = errors.New("queue is full")
	ErrQueueEmpty  = errors.New("queue is empty")
	ErrQueueClosed = errors.New("queue is closed")
)

type BoundedQueue struct {
	items    []interface{}
	capacity int
	size     int
	head     int
	tail     int
	mu       sync.Mutex
	notFull  *sync.Cond
	notEmpty *sync.Cond
	closed   bool
}

// Create a new bounded queue with specified capacity
func NewBoundedQueue(capacity int) *BoundedQueue {
	q := &BoundedQueue{
		items:    make([]interface{}, capacity),
		capacity: capacity,
	}
	q.notFull = sync.NewCond(&q.mu)
	q.notEmpty = sync.NewCond(&q.mu)
	return q
}

// Add an item to the queue, block if full
// Return error if queue is closed
func (q *BoundedQueue) Enqueue(item interface{}) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return ErrQueueClosed
	}
	for !q.closed && q.size == q.capacity {
		q.notFull.Wait()
	}

	q.items[q.tail] = item
	q.size++
	q.tail++
	if q.tail == q.capacity {
		q.tail = 0
	}

	q.notEmpty.Signal()
	return nil
}

// Remove and return an item from the queue, block if empty
// Return error if queue is closed
func (q *BoundedQueue) Dequeue() (interface{}, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return nil, ErrQueueClosed
	}
	for !q.closed && q.size == 0 {
		q.notEmpty.Wait()
	}
	if q.size == 0 && q.closed {
		return nil, ErrQueueClosed
	}

	ret := q.items[q.head]
	q.items[q.head] = nil
	q.size--
	q.head++
	if q.head == q.capacity {
		q.head = 0
	}
	q.notFull.Signal()
	return ret, nil
}

// Try to add an item without blocking
// Return true if successful, false if queue is full
func (q *BoundedQueue) TryEnqueue(item interface{}) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.size == q.capacity {
		return false
	}

	q.items[q.tail] = item
	q.size++
	q.tail++
	if q.tail == q.capacity {
		q.tail = 0
	}

	q.notEmpty.Signal()

	return true
}

// Try to remove an item without blocking
// Return item and true if successful, nil and false if queue is empty
func (q *BoundedQueue) TryDequeue() (interface{}, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.size == 0 {
		return nil, false
	}
	ret := q.items[q.head]
	q.items[q.head] = nil
	q.size--
	q.head++
	if q.head == q.capacity {
		q.head = 0
	}
	q.notFull.Signal()

	return ret, true
}

// Return the current number of items in the queue
func (q *BoundedQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size
}

// Close the queue and wake up all waiting goroutines
func (q *BoundedQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.notEmpty.Broadcast()
	q.notFull.Broadcast()
}

func producer(id int, queue *BoundedQueue, count int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < count; i++ {
		item := fmt.Sprintf("P%d-Item%d", id, i)
		if err := queue.Enqueue(item); err != nil {
			fmt.Printf("Producer %d: failed to enqueue: %v\n", id, err)
			return
		}
		fmt.Printf("Producer %d: enqueued %s (queue size: %d)\n", id, item, queue.Size())
		time.Sleep(100 * time.Millisecond)
	}
}

func consumer(id int, queue *BoundedQueue, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		item, err := queue.Dequeue()
		if err != nil {
			fmt.Printf("Consumer %d: stopping (%v)\n", id, err)
			return
		}
		fmt.Printf("Consumer %d: dequeued %s (queue size: %d)\n", id, item, queue.Size())
		time.Sleep(150 * time.Millisecond)
	}
}

func main() {
	queue := NewBoundedQueue(5)
	var wg sync.WaitGroup

	// Start 2 producers
	wg.Add(2)
	go producer(1, queue, 5, &wg)
	go producer(2, queue, 5, &wg)
	// Start 3 consumers
	wg.Add(3)
	go consumer(1, queue, &wg)
	go consumer(2, queue, &wg)
	go consumer(3, queue, &wg)

	// Wait for producers to finish
	time.Sleep(2 * time.Second)

	// Close queue and wait for consumers to finish
	fmt.Println("\nClosing queue...")
	queue.Close()
	wg.Wait()

	fmt.Println("All goroutines finished")
}

// Test: Run with `go run main.go` and `go test -race`
// Expected: Producers block when queue is full
// Consumers block when queue is empty
// All items produced are consumed
// Clean shutdown after Close()
// No race conditions, no deadlocks
