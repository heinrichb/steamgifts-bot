package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestWaitRespectsBounds(t *testing.T) {
	l := New(5*time.Millisecond, 15*time.Millisecond)
	for i := 0; i < 10; i++ {
		start := time.Now()
		if err := l.Wait(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		elapsed := time.Since(start)
		if elapsed < 5*time.Millisecond {
			t.Errorf("wait %d: %s shorter than min", i, elapsed)
		}
		if elapsed > 50*time.Millisecond {
			t.Errorf("wait %d: %s much longer than max (scheduler jitter ok, but this is too high)", i, elapsed)
		}
	}
}

func TestWaitCancellable(t *testing.T) {
	l := New(time.Hour, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.Wait(ctx); err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestZeroDurationReturnsImmediately(t *testing.T) {
	l := New(0, 0)
	start := time.Now()
	_ = l.Wait(context.Background())
	if d := time.Since(start); d > 5*time.Millisecond {
		t.Errorf("zero limiter slept for %s", d)
	}
}
