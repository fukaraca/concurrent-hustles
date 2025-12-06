package concurrent_hustles

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
