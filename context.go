package main

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

// Context represents a custom context implementation
// It should carry deadlines, cancellation signals, and request-scoped values
type Context interface {
	// Deadline returns the time when work done on behalf of this context
	// should be canceled. Returns ok==false when no deadline is set.
	Deadline() (deadline time.Time, ok bool)

	// Done returns a channel that's closed when work done on behalf of
	// this context should be canceled.
	Done() <-chan struct{}

	// Err returns a non-nil error value after Done is closed.
	// Err returns Canceled if the context was canceled
	// or DeadlineExceeded if the context's deadline passed.
	Err() error

	// Value returns the value associated with this context for key,
	// or nil if no value is associated with key.
	Value(key any) any
}

// CancelFunc tells an operation to abandon its work.
type CancelFunc func()

// Common errors
var (
	Canceled         = fmt.Errorf("context canceled")
	DeadlineExceeded = fmt.Errorf("context deadline exceeded")
)

// emptyCtx is the base context that is never canceled, has no values, and has no deadline
type emptyCtx int

// Should return zero time and false
func (e emptyCtx) Deadline() (deadline time.Time, ok bool) {
	return
}

// Should return nil channel (never done)
func (e emptyCtx) Done() <-chan struct{} {
	return nil
}

// Should return nil (never has an error)
func (e emptyCtx) Err() error {
	return nil
}

// Should return nil (no values stored)
func (e emptyCtx) Value(key any) any {
	return nil
}

var (
	background = new(emptyCtx)
	todo       = new(emptyCtx)
)

// Background returns a non-nil, empty Context.
// It is never canceled, has no values, and has no deadline.
func Background() Context {
	return background
}

// Use when you're unsure about which Context to use or if it's not yet available.
func TODO() Context {
	return todo
}

// cancelCtx represents a context that can be canceled
type cancelCtx struct {
	Context
	done chan struct{}
	mu   sync.Mutex
	err  error
}

// Creates a copy of parent with a new Done channel.
// The returned context's Done channel is closed when:
// - the returned cancel function is called, or
// - the parent context's Done channel is closed
// whichever happens first.
func WithCancel(parent Context) (Context, CancelFunc) {
	ctx := &cancelCtx{
		Context: parent,
		done:    make(chan struct{}),
	}

	cancel := ctx.cancel(Canceled)

	go func() {
		select {
		case <-parent.Done():
			ctx.cancel(parent.Err())()
		case <-ctx.Done():

		}
	}()

	return ctx, cancel
}

func (c *cancelCtx) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *cancelCtx) Done() <-chan struct{} {
	return c.done
}

func (c *cancelCtx) cancel(err error) CancelFunc {
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.err != nil {
			return
		}
		c.err = err
		close(c.done)
	}
}

type deadlineCtx struct {
	*cancelCtx
	deadline time.Time
}

func WithDeadline(parent Context, deadline time.Time) (Context, CancelFunc) {
	cctx, cancel := WithCancel(parent)
	ctx := &deadlineCtx{
		cancelCtx: cctx.(*cancelCtx),
		deadline:  deadline,
	}
	t := time.AfterFunc(time.Until(deadline), func() {
		ctx.cancel(DeadlineExceeded)()
	})
	stop := func() {
		t.Stop()
		cancel()
	}
	return ctx, stop
}

func (c *deadlineCtx) Deadline() (time.Time, bool) {
	return c.deadline, true
}

func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
	return WithDeadline(parent, time.Now().Add(timeout))
}

type valueCtx struct {
	Context
	key, val any
}

func (c *valueCtx) Value(key any) any {
	if c.key == key {
		return c.val
	}
	return c.Context.Value(key)
}

func WithValue(parent Context, key, val any) Context {
	if key == nil {
		panic("key cannot be nil")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("key should be comparable")
	}
	ctx := &valueCtx{
		Context: parent,
		key:     key,
		val:     val,
	}
	return ctx
}

// ==================== MAIN FUNCTION WITH TEST SCENARIOS ====================

func main() {
	fmt.Println("=== Custom Context Package Tests ===\n")

	// Test 1: Background and TODO contexts
	fmt.Println("Test 1: Background and TODO contexts")
	testBackgroundAndTODO()
	fmt.Println()

	// Test 2: WithCancel - manual cancellation
	fmt.Println("Test 2: WithCancel - manual cancellation")
	testWithCancel()
	fmt.Println()

	// Test 3: WithCancel - parent cancellation propagation
	fmt.Println("Test 3: WithCancel - parent cancellation propagation")
	testParentCancellationPropagation()
	fmt.Println()

	// Test 4: WithTimeout - timeout occurs
	fmt.Println("Test 4: WithTimeout - timeout occurs")
	testWithTimeout()
	fmt.Println()

	// Test 5: WithTimeout - cancel before timeout
	fmt.Println("Test 5: WithTimeout - cancel before timeout")
	testCancelBeforeTimeout()
	fmt.Println()

	// Test 6: WithDeadline
	fmt.Println("Test 6: WithDeadline")
	testWithDeadline()
	fmt.Println()

	// Test 7: WithValue
	fmt.Println("Test 7: WithValue")
	testWithValue()
	fmt.Println()

	// Test 8: Complex scenario - multiple goroutines with cancellation
	fmt.Println("Test 8: Complex scenario - multiple goroutines")
	testComplexScenario()
	fmt.Println()

	// Test 9: Context tree with mixed types
	fmt.Println("Test 9: Context tree with mixed types")
	testContextTree()
	fmt.Println()

	fmt.Println("=== All Tests Complete ===")
}

func testBackgroundAndTODO() {
	bg := Background()
	td := TODO()

	fmt.Printf("Background Done channel is nil: %v\n", bg.Done() == nil)
	fmt.Printf("Background Err: %v\n", bg.Err())
	fmt.Printf("TODO Done channel is nil: %v\n", td.Done() == nil)
	fmt.Printf("TODO Err: %v\n", td.Err())

	_, bgHasDeadline := bg.Deadline()
	_, tdHasDeadline := td.Deadline()
	fmt.Printf("Background has deadline: %v\n", bgHasDeadline)
	fmt.Printf("TODO has deadline: %v\n", tdHasDeadline)
}

func testWithCancel() {
	ctx, cancel := WithCancel(Background())

	fmt.Println("Before cancel:")
	fmt.Printf("  Err: %v\n", ctx.Err())

	go func() {
		<-ctx.Done()
		fmt.Println("Context done signal received!")
		fmt.Printf("  Err after Done: %v\n", ctx.Err())
	}()

	time.Sleep(100 * time.Millisecond)
	fmt.Println("Calling cancel()...")
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func testParentCancellationPropagation() {
	parent, cancelParent := WithCancel(Background())
	child, _ := WithCancel(parent)
	grandchild, _ := WithCancel(child)

	done := make(chan string, 3)

	go func() {
		<-grandchild.Done()
		done <- "grandchild"
	}()

	go func() {
		<-child.Done()
		done <- "child"
	}()

	go func() {
		<-parent.Done()
		done <- "parent"
	}()

	time.Sleep(100 * time.Millisecond)
	fmt.Println("Canceling parent...")
	cancelParent()

	// Collect results
	for i := 0; i < 3; i++ {
		select {
		case name := <-done:
			fmt.Printf("  %s received cancellation\n", name)
		case <-time.After(200 * time.Millisecond):
			fmt.Println("  Timeout waiting for cancellation")
		}
	}
}

func testWithTimeout() {
	ctx, cancel := WithTimeout(Background(), 200*time.Millisecond)
	defer cancel()

	fmt.Println("Waiting for timeout (200ms)...")
	start := time.Now()

	<-ctx.Done()
	elapsed := time.Since(start)

	fmt.Printf("Context done after %v\n", elapsed)
	fmt.Printf("Error: %v\n", ctx.Err())
}

func testCancelBeforeTimeout() {
	ctx, cancel := WithTimeout(Background(), 500*time.Millisecond)

	fmt.Println("Setting timeout to 500ms, but canceling after 100ms...")
	start := time.Now()

	go func() {
		time.Sleep(100 * time.Millisecond)
		fmt.Println("Calling cancel()...")
		cancel()
	}()

	<-ctx.Done()
	elapsed := time.Since(start)

	fmt.Printf("Context done after %v\n", elapsed)
	fmt.Printf("Error: %v\n", ctx.Err())
}

func testWithDeadline() {
	deadline := time.Now().Add(150 * time.Millisecond)
	ctx, cancel := WithDeadline(Background(), deadline)
	defer cancel()

	fmt.Printf("Deadline set to %v from now\n", time.Until(deadline))

	dl, ok := ctx.Deadline()
	fmt.Printf("Context deadline: %v, has deadline: %v\n", dl, ok)

	<-ctx.Done()
	fmt.Printf("Error: %v\n", ctx.Err())
}

func testWithValue() {
	ctx := WithValue(Background(), key("user"), "alice")
	ctx = WithValue(ctx, key("requestID"), "12345")
	ctx = WithValue(ctx, key("role"), "admin")

	fmt.Printf("user: %v\n", ctx.Value(key("user")))
	fmt.Printf("requestID: %v\n", ctx.Value(key("requestID")))
	fmt.Printf("role: %v\n", ctx.Value(key("role")))
	fmt.Printf("nonexistent: %v\n", ctx.Value(key("nonexistent")))
}

type key string

func testComplexScenario() {
	// Simulate a request handler that spawns multiple workers

	ctx := WithValue(Background(), key("requestID"), "req-789")
	ctx, cancel := WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup

	// Spawn 3 worker goroutines
	for i := 1; i <= 3; i++ {
		wg.Add(1)
		workerID := i

		go func() {
			defer wg.Done()
			worker2(ctx, workerID)
		}()
	}

	wg.Wait()
	fmt.Println("All workers completed")
}

func worker2(ctx Context, id int) {
	reqID := ctx.Value(key("requestID"))
	fmt.Printf("Worker %d started (requestID: %v)\n", id, reqID)

	// Simulate work with periodic checks
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Worker %d stopping due to: %v\n", id, ctx.Err())
			return
		case <-ticker.C:
			fmt.Printf("Worker %d working...\n", id)
		}
	}
}

func testContextTree() {
	// Create a complex context tree
	root := WithValue(Background(), key("level"), "root")

	branch1, cancel1 := WithTimeout(root, 300*time.Millisecond)
	branch1 = WithValue(branch1, key("branch"), "1")

	branch2, cancel2 := WithCancel(root)
	branch2 = WithValue(branch2, key("branch"), "2")

	leaf1, cancel3 := WithCancel(branch1)
	leaf1 = WithValue(leaf1, key("leaf"), "1")

	fmt.Printf("Leaf1 level: %v\n", leaf1.Value(key("level")))
	fmt.Printf("Leaf1 branch: %v\n", leaf1.Value(key("branch")))
	fmt.Printf("Leaf1 leaf: %v\n", leaf1.Value(key("leaf")))

	// Test cancellation
	go func() {
		<-leaf1.Done()
		fmt.Println("Leaf1 done!")
	}()

	go func() {
		<-branch2.Done()
		fmt.Println("Branch2 done!")
	}()

	time.Sleep(100 * time.Millisecond)
	fmt.Println("Canceling branch2...")
	cancel2()

	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel1()
	cancel3()
	time.Sleep(100 * time.Millisecond)
}
