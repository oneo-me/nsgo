// Package queue hands mount batches from the engine goroutine to a
// platform's main thread.
package queue

import (
	"sync"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Batches is a queue the engine pushes to and the main thread drains.
type Batches struct {
	mu      sync.Mutex
	pending []*driver.MountBatch
}

// Push appends b and reports whether the queue was empty, so the caller
// wakes the main thread once per drain instead of once per batch.
func (q *Batches) Push(b *driver.MountBatch) (wake bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending = append(q.pending, b)
	return len(q.pending) == 1
}

// Len returns the number of queued batches.
func (q *Batches) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pending)
}

// Drain returns and removes every queued batch.
func (q *Batches) Drain() []*driver.MountBatch {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := q.pending
	q.pending = nil
	return out
}
