package testfixtures

import (
	"fmt"
	"sync"
)

// IDGenerator produces deterministic identifiers for tests.
type IDGenerator struct {
	mu      sync.Mutex
	prefix  string
	counter uint64
}

// NewIDGenerator constructs a generator that yields identifiers with the given
// prefix. When prefix is empty, "id" is used.
func NewIDGenerator(prefix string) *IDGenerator {
	if prefix == "" {
		prefix = "id"
	}
	return &IDGenerator{prefix: prefix}
}

// Next returns the next identifier in the sequence.
func (g *IDGenerator) Next() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	return fmt.Sprintf("%s-%d", g.prefix, g.counter)
}

// NextFunc exposes Next as a function suitable for dependency injection.
func (g *IDGenerator) NextFunc() func() string {
	if g == nil {
		return func() string { return "" }
	}
	return g.Next
}

// SetPrefix updates the generator prefix.
func (g *IDGenerator) SetPrefix(prefix string) {
	g.mu.Lock()
	g.prefix = prefix
	g.mu.Unlock()
}

// SetCounter overrides the internal counter, enabling deterministic resets.
func (g *IDGenerator) SetCounter(counter uint64) {
	g.mu.Lock()
	g.counter = counter
	g.mu.Unlock()
}
