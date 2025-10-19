package testfixtures

import (
	"sync"
	"time"
)

// Clock provides a controllable time source for tests.
type Clock struct {
	mu      sync.Mutex
	current time.Time
}

// NewClock returns a clock initialised to the supplied time. When start is the
// zero value, the shared ReferenceTime is used.
func NewClock(start time.Time) *Clock {
	if start.IsZero() {
		start = ReferenceTime()
	}
	return &Clock{current: start}
}

// Now returns the current instant tracked by the clock.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

// NowFunc exposes Now as a function suitable for dependency injection.
func (c *Clock) NowFunc() func() time.Time {
	if c == nil {
		return time.Now
	}
	return c.Now
}

// Set updates the clock to the provided time.
func (c *Clock) Set(t time.Time) {
	c.mu.Lock()
	c.current = t
	c.mu.Unlock()
}

// Advance moves the clock forward by the provided duration and returns the
// updated time.
func (c *Clock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	c.current = c.current.Add(d)
	updated := c.current
	c.mu.Unlock()
	return updated
}

// Current returns the clock time without modifying it. It is equivalent to
// calling Now but signals the absence of time progression.
func (c *Clock) Current() time.Time {
	return c.Now()
}
