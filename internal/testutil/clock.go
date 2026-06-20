// Package testutil holds small, dependency-free helpers shared across the
// module's tests. It imports only the standard library and must never be
// imported by production code.
package testutil

import (
	"sync"
	"time"
)

// FakeClock is a controllable, concurrency-safe core.Clock implementation for
// tests. It satisfies the core.Clock seam (Now() time.Time) structurally. Time
// never advances on its own: tests move it with Set or Advance, keeping
// time-dependent behavior deterministic.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewFakeClock returns a FakeClock anchored at t.
func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{now: t}
}

// Now returns the clock's current instant. It is safe for concurrent use.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Set moves the clock to t.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// Advance moves the clock forward by d (negative d moves backward).
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}
