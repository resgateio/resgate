package rescache

import "sync"

// Throttle ensures that only a set number of callbacks are running at the same
// time. Once a callback is complete, it should call Done to let next queued
// callback to run.
//
// Throttles runs all callbacks on the same goroutine. It is expected that the
// callback in itself passes the call to another goroutine.
type Throttle struct {
	limit   int
	running int
	mu      sync.Mutex
	queue   []func()
}

// NewThrottle creates a new throttle.
func NewThrottle(limit int) *Throttle {
	return &Throttle{limit: limit}
}

// Add calls the provided callback or queues it if the limit of concurrently
// running callbacks is reached.
func (t *Throttle) Add(cb func()) {
	t.mu.Lock()

	if t.running >= t.limit {
		t.queue = append(t.queue, cb)
		t.mu.Unlock()
		return
	}
	t.running++
	t.mu.Unlock()

	cb()
}

// Done marks a call to be done, allowing for the next queued callback to be
// called.
func (t *Throttle) Done() {
	if t == nil {
		return
	}
	t.mu.Lock()

	if t.running <= 0 {
		t.mu.Unlock()
		panic("throttle: negative running counter")
	}

	if len(t.queue) == 0 {
		t.running--
		t.mu.Unlock()
		return
	}

	cb := t.queue[0]
	t.queue = t.queue[1:]
	t.mu.Unlock()
	cb()
}
