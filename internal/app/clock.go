package app

import (
	"sync"
	"time"
)

// Clock abstracts time so that tests can advance the clock deterministically
// without sleeping.
type Clock interface {
	Now() time.Time
}

// realClock returns the wall-clock time.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// RealClock returns a Clock backed by time.Now.
func RealClock() Clock { return realClock{} }

// FakeClock is a controllable clock for testing. It is safe for concurrent use.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewFakeClock creates a FakeClock set to the given instant.
func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{now: t}
}

// Now returns the current fake time.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves the fake clock forward by d.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// Set replaces the fake clock's current time.
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}
