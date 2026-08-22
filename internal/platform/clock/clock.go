package clock

import (
	"sync"
	"sync/atomic"
	"time"
)

type Clock interface {
	Now() time.Time
}

type MonotonicUTC struct {
	lastUnixNano atomic.Int64
}

func (c *MonotonicUTC) Now() time.Time {
	observed := time.Now().UTC().UnixNano()
	for {
		previous := c.lastUnixNano.Load()
		next := observed
		if next <= previous {
			next = previous + 1
		}
		if c.lastUnixNano.CompareAndSwap(previous, next) {
			return time.Unix(0, next).UTC()
		}
	}
}

type Fixed struct {
	mu  sync.RWMutex
	now time.Time
}

func NewFixed(now time.Time) *Fixed {
	return &Fixed{now: now.UTC()}
}

func (f *Fixed) Now() time.Time {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.now
}

func (f *Fixed) Advance(duration time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(duration)
	f.mu.Unlock()
}
