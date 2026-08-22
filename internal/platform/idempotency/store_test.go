package idempotency

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentBeginAllowsOnlyOneOwner(t *testing.T) {
	store := NewMemory()
	start := make(chan struct{})
	var owners atomic.Int32
	var workers sync.WaitGroup
	for i := 0; i < 16; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, replay, err := store.Begin(context.Background(), "telemetry", "event-1", "same", time.Now().Add(time.Minute))
			if err == nil && !replay {
				owners.Add(1)
			}
		}()
	}
	close(start)
	workers.Wait()
	if owners.Load() != 1 {
		t.Fatalf("owners = %d, want 1", owners.Load())
	}
}
