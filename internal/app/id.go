package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
)

// IDGenerator produces unique identifiers for domain objects.
type IDGenerator interface {
	Next(prefix string) string
}

// atomicIDGen uses a monotonically increasing counter prefixed by a short
// random hex string, guaranteeing uniqueness across restarts in practice.
type atomicIDGen struct {
	counter uint64
	randHex string
}

// NewAtomicIDGenerator returns an IDGenerator suitable for production.
func NewAtomicIDGenerator() IDGenerator {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return &atomicIDGen{randHex: hex.EncodeToString(b)}
}

func (g *atomicIDGen) Next(prefix string) string {
	n := atomic.AddUint64(&g.counter, 1)
	return fmt.Sprintf("%s-%s-%06d", prefix, g.randHex, n)
}

// sequentialIDGen returns deterministic IDs for testing.
type sequentialIDGen struct {
	counter uint64
}

// NewSequentialIDGenerator returns an IDGenerator that produces IDs like
// "prefix-1", "prefix-2", …
func NewSequentialIDGenerator() IDGenerator {
	return &sequentialIDGen{}
}

func (g *sequentialIDGen) Next(prefix string) string {
	n := atomic.AddUint64(&g.counter, 1)
	return fmt.Sprintf("%s-%d", prefix, n)
}
