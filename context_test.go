// context_test.go
package concurrent_hustles

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// --- Helpers --------------------------------------------------------------
func workerHelper(ctx Context, id int) {
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

func waitDone(t *testing.T, ctx Context, d time.Duration) error {
	t.Helper()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		t.Fatalf("context did not finish within %v", d)
		return nil
	}
}

// --- Test 1: Background and TODO -----------------------------------------

func TestBackgroundAndTODO(t *testing.T) {
	bg := Background()
	td := TODO()

	if bg == nil {
		t.Fatalf("Background returned nil")
	}
	if td == nil {
		t.Fatalf("TODO returned nil")
	}

	if bg.Done() != nil {
		t.Errorf("Background.Done() should be nil (never done)")
	}
	if td.Done() != nil {
		t.Errorf("TODO.Done() should be nil (never done)")
	}

	if bg.Err() != nil {
		t.Errorf("Background.Err() must be nil, got %v", bg.Err())
	}
	if td.Err() != nil {
		t.Errorf("TODO.Err() must be nil, got %v", td.Err())
	}

	if dl, ok := bg.Deadline(); !dl.IsZero() || ok {
		t.Errorf("Background should have no deadline, got %v, ok=%v", dl, ok)
	}
	if dl, ok := td.Deadline(); !dl.IsZero() || ok {
		t.Errorf("TODO should have no deadline, got %v, ok=%v", dl, ok)
	}
}

// --- Test 2: WithCancel - manual cancellation ----------------------------

func TestWithCancel_Manual(t *testing.T) {
	ctx, cancel := WithCancel(Background())
	defer cancel()

	if ctx.Err() != nil {
		t.Fatalf("Err should be nil before cancel, got %v", ctx.Err())
	}

	select {
	case <-ctx.Done():
		t.Fatalf("Done should not be closed before cancel")
	default:
	}

	cancel()

	select {
	case <-ctx.Done():
		// ok
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("Done was not closed after cancel")
	}

	if ctx.Err() != Canceled {
		t.Fatalf("Err should be Canceled after cancel, got %v", ctx.Err())
	}

	// Multiple cancels must be safe and idempotent
	cancel()
	cancel()

	if ctx.Err() != Canceled {
		t.Fatalf("Err should remain Canceled after multiple cancels, got %v", ctx.Err())
	}
}

// --- Test 3: WithCancel - parent cancellation propagation ----------------

func TestWithCancel_ParentPropagation(t *testing.T) {
	parent, cancelParent := WithCancel(Background())
	child, _ := WithCancel(parent)
	grandchild, _ := WithCancel(child)

	// Ensure not canceled yet
	select {
	case <-parent.Done():
		t.Fatalf("parent should not be canceled yet")
	default:
	}

	cancelParent()

	for name, ctx := range map[string]Context{
		"parent":     parent,
		"child":      child,
		"grandchild": grandchild,
	} {
		select {
		case <-ctx.Done():
			if ctx.Err() != Canceled {
				t.Errorf("%s.Err() = %v, want %v", name, ctx.Err(), Canceled)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("%s did not receive cancellation from parent", name)
		}
	}
}

// --- Test 4: WithTimeout - timeout occurs -------------------------------

func TestWithTimeout_DeadlineExceeded(t *testing.T) {
	ctx, cancel := WithTimeout(Background(), 40*time.Millisecond)
	defer cancel()

	start := time.Now()
	<-ctx.Done()
	elapsed := time.Since(start)

	if ctx.Err() != DeadlineExceeded {
		t.Fatalf("Err = %v, want DeadlineExceeded", ctx.Err())
	}

	if elapsed < 30*time.Millisecond {
		t.Fatalf("timeout fired too early: %v", elapsed)
	}
}

// --- Test 5: WithTimeout - cancel before timeout ------------------------

func TestWithTimeout_CancelBeforeTimeout(t *testing.T) {
	ctx, cancel := WithTimeout(Background(), 500*time.Millisecond)

	start := time.Now()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	<-ctx.Done()
	elapsed := time.Since(start)

	if ctx.Err() != Canceled {
		t.Fatalf("Err = %v, want Canceled", ctx.Err())
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("context finished too late after cancel: %v", elapsed)
	}
}

// --- Test 6: WithDeadline ------------------------------------------------

func TestWithDeadline(t *testing.T) {
	deadline := time.Now().Add(60 * time.Millisecond)
	ctx, cancel := WithDeadline(Background(), deadline)
	defer cancel()

	dl, ok := ctx.Deadline()
	if !ok {
		t.Fatalf("Deadline() ok=false, expected true")
	}
	if !dl.Equal(deadline) {
		t.Fatalf("Deadline mismatch: got %v, want %v", dl, deadline)
	}

	err := waitDone(t, ctx, 300*time.Millisecond)
	if err != DeadlineExceeded {
		t.Fatalf("Err = %v, want DeadlineExceeded", err)
	}
}

func TestWithDeadline_CancelBeforeDeadline(t *testing.T) {
	deadline := time.Now().Add(500 * time.Millisecond)
	ctx, cancel := WithDeadline(Background(), deadline)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := waitDone(t, ctx, 300*time.Millisecond)
	if err != Canceled {
		t.Fatalf("Err = %v, want Canceled when user cancels before deadline", err)
	}
}

// --- Test 7: WithValue ---------------------------------------------------

type key string

func TestWithValue_BasicChaining(t *testing.T) {
	ctx := WithValue(Background(), key("user"), "alice")
	ctx = WithValue(ctx, key("requestID"), "12345")
	ctx = WithValue(ctx, key("role"), "admin")

	if got := ctx.Value(key("user")); got != "alice" {
		t.Fatalf("user = %v, want alice", got)
	}
	if got := ctx.Value(key("requestID")); got != "12345" {
		t.Fatalf("requestID = %v, want 12345", got)
	}
	if got := ctx.Value(key("role")); got != "admin" {
		t.Fatalf("role = %v, want admin", got)
	}
	if got := ctx.Value(key("nonexistent")); got != nil {
		t.Fatalf("nonexistent = %v, want nil", got)
	}
}

func TestWithValue_Shadowing(t *testing.T) {
	root := WithValue(Background(), key("user"), "alice")
	child := WithValue(root, key("user"), "bob")

	if got := root.Value(key("user")); got != "alice" {
		t.Fatalf("root user = %v, want alice", got)
	}
	if got := child.Value(key("user")); got != "bob" {
		t.Fatalf("child user = %v, want bob (shadowed)", got)
	}
}

func TestWithValue_PanicsOnNilKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("WithValue should panic when key is nil")
		}
	}()
	_ = WithValue(Background(), nil, "x")
}

func TestWithValue_PanicsOnNonComparableKey(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("WithValue should panic when key is non-comparable")
		}
	}()
	_ = WithValue(Background(), []int{1, 2, 3}, "x")
}

// --- Test 8: Complex scenario - multiple goroutines ----------------------
// Mirrors testComplexScenario() semantics without relying on prints.

func TestWorkersRespectCancellation(t *testing.T) {
	ctx := WithValue(Background(), key("requestID"), "req-789")
	ctx, cancel := WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	const workers = 3
	wg.Add(workers)

	for i := 1; i <= workers; i++ {
		id := i
		go func() {
			defer wg.Done()
			workerHelper(ctx, id)
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// all workers exited after ctx.Done
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("workers did not finish after context cancellation")
	}
}

// --- Test 9: Context tree with mixed types -------------------------------

func TestContextTree_ValuesAndCancellation(t *testing.T) {
	root := WithValue(Background(), key("level"), "root")

	branch1, cancel1 := WithTimeout(root, 60*time.Millisecond)
	defer cancel1()
	branch1 = WithValue(branch1, key("branch"), "1")

	branch2, cancel2 := WithCancel(root)
	defer cancel2()
	branch2 = WithValue(branch2, key("branch"), "2")

	leaf1, cancel3 := WithCancel(branch1)
	defer cancel3()
	leaf1 = WithValue(leaf1, key("leaf"), "1")

	// Value lookup along the chain
	if got := leaf1.Value(key("level")); got != "root" {
		t.Fatalf("leaf1 level = %v, want root", got)
	}
	if got := leaf1.Value(key("branch")); got != "1" {
		t.Fatalf("leaf1 branch = %v, want 1", got)
	}
	if got := leaf1.Value(key("leaf")); got != "1" {
		t.Fatalf("leaf1 leaf = %v, want 1", got)
	}

	// Cancel branch2 explicitly: should not affect branch1/leaf1.
	cancel2()

	select {
	case <-branch2.Done():
		if branch2.Err() != Canceled {
			t.Fatalf("branch2 Err = %v, want Canceled", branch2.Err())
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("branch2 not canceled in time")
	}

	select {
	case <-leaf1.Done():
		t.Fatalf("leaf1 should not be canceled yet by branch2 cancel")
	case <-time.After(20 * time.Millisecond):
		// still alive, good
	}

	// Now let branch1 timeout and propagate to leaf1
	err := waitDone(t, branch1, 300*time.Millisecond)
	if err != DeadlineExceeded {
		t.Fatalf("branch1 Err = %v, want DeadlineExceeded", err)
	}

	err = waitDone(t, leaf1, 300*time.Millisecond)
	if err != DeadlineExceeded {
		t.Fatalf("leaf1 Err = %v, want DeadlineExceeded", err)
	}
}

// --- Test 10: Concurrency / race-ish scenario for WithCancel -------------

func TestWithCancel_ConcurrentCancelAndWaiters(t *testing.T) {
	ctx, cancel := WithCancel(Background())

	var wg sync.WaitGroup
	const goroutines = 16
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				// multiple concurrent cancels
				cancel()
			} else {
				// waiters racing with cancel
				<-ctx.Done()
				if ctx.Err() == nil {
					t.Errorf("waiter saw nil Err after Done closed")
				}
			}
		}()
	}

	wg.Wait()

	if ctx.Err() == nil {
		t.Fatalf("final Err should be non-nil after concurrent cancel")
	}
}
