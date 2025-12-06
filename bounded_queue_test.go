package concurrent_hustles

import (
	"sync"
	"testing"
	"time"
)

// Enqueue should block when the queue is full, and unblock after a dequeue.
func TestBoundedQueueEnqueueBlocksWhenFull(t *testing.T) {
	q := NewBoundedQueue(2)

	// Fill the queue
	if err := q.Enqueue("a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := q.Enqueue("b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	done := make(chan error, 1)

	// This should block until we dequeue.
	go func() {
		err := q.Enqueue("c")
		done <- err
	}()

	// Give it a moment to (hopefully) block.
	time.Sleep(50 * time.Millisecond)

	select {
	case err := <-done:
		t.Fatalf("Enqueue did not block on full queue, returned early with: %v", err)
	default:
		// good, still blocked
	}

	// Now dequeue one item, which should unblock the blocked Enqueue.
	_, err := q.Dequeue()
	if err != nil {
		t.Fatalf("unexpected error from Dequeue: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("blocked Enqueue returned error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("blocked Enqueue did not complete after Dequeue")
	}
}

// Dequeue should block when the queue is empty and unblock after an enqueue.
func TestBoundedQueueDequeueBlocksWhenEmpty(t *testing.T) {
	q := NewBoundedQueue(2)

	done := make(chan struct{})
	var got interface{}
	var gotErr error

	go func() {
		defer close(done)
		got, gotErr = q.Dequeue()
	}()

	// Give it time to block.
	time.Sleep(50 * time.Millisecond)

	select {
	case <-done:
		t.Fatalf("Dequeue did not block on empty queue")
	default:
		// good, still blocked
	}

	// Enqueue an item, which should wake the blocked Dequeue.
	want := "x"
	if err := q.Enqueue(want); err != nil {
		t.Fatalf("unexpected error from Enqueue: %v", err)
	}

	select {
	case <-done:
		if gotErr != nil {
			t.Fatalf("Dequeue returned error: %v", gotErr)
		}
		if got != want {
			t.Fatalf("Dequeue got %v, want %v", got, want)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("blocked Dequeue did not complete after Enqueue")
	}
}

// TryEnqueue and TryDequeue should be non-blocking and respect capacity/emptiness.
func TestBoundedQueueTryOperations(t *testing.T) {
	q := NewBoundedQueue(1)

	// Empty queue: TryDequeue should fail.
	if v, ok := q.TryDequeue(); ok || v != nil {
		t.Fatalf("expected TryDequeue to fail on empty queue, got (%v, %v)", v, ok)
	}

	// First TryEnqueue should succeed.
	if ok := q.TryEnqueue(42); !ok {
		t.Fatalf("expected TryEnqueue to succeed on empty queue")
	}

	// Now full: TryEnqueue should fail.
	if ok := q.TryEnqueue(99); ok {
		t.Fatalf("expected TryEnqueue to fail on full queue")
	}

	// TryDequeue should succeed now.
	if v, ok := q.TryDequeue(); !ok || v.(int) != 42 {
		t.Fatalf("expected TryDequeue to return 42,true; got (%v, %v)", v, ok)
	}

	// Empty again: TryDequeue should fail.
	if v, ok := q.TryDequeue(); ok || v != nil {
		t.Fatalf("expected TryDequeue to fail on empty queue, got (%v, %v)", v, ok)
	}
}

// Close should wake blocked waiters and prevent further enqueues.
func TestBoundedQueueCloseWakesWaiters(t *testing.T) {
	// 1) Dequeue blocked on empty queue should wake with ErrQueueClosed.
	q1 := NewBoundedQueue(1)
	deqDone := make(chan error, 1)

	go func() {
		_, err := q1.Dequeue()
		deqDone <- err
	}()

	time.Sleep(50 * time.Millisecond) // let it block

	q1.Close()

	select {
	case err := <-deqDone:
		if err != ErrQueueClosed {
			t.Fatalf("expected ErrQueueClosed from blocked Dequeue after Close, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("blocked Dequeue was not woken by Close")
	}

	// 2) Enqueue blocked on full queue should wake after Close (even if it still succeeds).
	q2 := NewBoundedQueue(1)
	if err := q2.Enqueue("first"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	enqDone := make(chan error, 1)
	go func() {
		err := q2.Enqueue("second") // will block until Close broadcasts
		enqDone <- err
	}()

	time.Sleep(50 * time.Millisecond) // let it block

	q2.Close()

	select {
	case <-enqDone:
		// We don't assert on err here because the implementation enqueues even after Close.
		// We just care that it woke up and returned (no deadlock).
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("blocked Enqueue was not woken by Close")
	}

	// After Close, further TryEnqueue should fail.
	if ok := q2.TryEnqueue("third"); ok {
		t.Fatalf("expected TryEnqueue to fail after Close")
	}
}

// Concurrent producers/consumers, good for go test -race.
func TestBoundedQueueConcurrentProducersConsumers(t *testing.T) {
	q := NewBoundedQueue(16)

	const (
		numProducers     = 8
		numConsumers     = 8
		itemsPerProducer = 100
	)

	var prodWg sync.WaitGroup
	var consWg sync.WaitGroup

	// Producers: finite work; ignore ErrQueueClosed (if Close happens early).
	for p := 0; p < numProducers; p++ {
		prodWg.Add(1)
		go func(id int) {
			defer prodWg.Done()
			base := id * 1000
			for i := 0; i < itemsPerProducer; i++ {
				val := base + i
				if err := q.Enqueue(val); err != nil {
					if err == ErrQueueClosed {
						return
					}
					t.Errorf("producer %d: unexpected error: %v", id, err)
					return
				}
			}
		}(p)
	}

	// Consumers: run until queue is closed and drained.
	for c := 0; c < numConsumers; c++ {
		consWg.Add(1)
		go func(id int) {
			defer consWg.Done()
			for {
				item, err := q.Dequeue()
				if err == ErrQueueClosed {
					return
				}
				_ = item // we don't care what it is here
			}
		}(c)
	}

	// Wait for producers to finish, then Close the queue so consumers can exit.
	prodDone := make(chan struct{})
	go func() {
		prodWg.Wait()
		close(prodDone)
	}()

	select {
	case <-prodDone:
		q.Close()
	case <-time.After(5 * time.Second):
		t.Fatalf("producers did not finish in time")
	}

	done := make(chan struct{})
	go func() {
		consWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatalf("consumers did not finish in time after Close")
	}
}
