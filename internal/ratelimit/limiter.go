// Package ratelimit provides a small jittered sleeper for spacing out
// HTTP requests against steamgifts.com.
//
// We deliberately do not use golang.org/x/time/rate: the steamgifts site
// is happiest with human-paced, irregular timing rather than precise token
// buckets. A min/max window plus uniform jitter mimics that well and keeps
// the dependency footprint minimal.
package ratelimit

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"
)

// Limiter delays callers between Min and Max, picking a uniformly-random
// duration on each Wait. It is safe for concurrent use.
type Limiter struct {
	mu  sync.Mutex
	min time.Duration
	max time.Duration
	rng *rand.Rand
}

// New constructs a Limiter. If max <= min, the limiter sleeps for exactly min.
// If min is negative, it is clamped to zero.
func New(min, max time.Duration) *Limiter {
	if min < 0 {
		min = 0
	}
	if max < min {
		max = min
	}
	return &Limiter{
		min: min,
		max: max,
		rng: rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0xC0FFEE)),
	}
}

// Wait blocks for a jittered duration in [min, max], or until ctx is done.
func (l *Limiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	span := l.max - l.min
	d := l.min
	if span > 0 {
		d += time.Duration(l.rng.Int64N(int64(span)))
	}
	l.mu.Unlock()

	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
