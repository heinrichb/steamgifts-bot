package state

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestLoadMissingFileReturnsEmptyStore(t *testing.T) {
	p := filepath.Join(t.TempDir(), "missing", "state.json")
	s, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := s.LastSync("anyone"); !got.IsZero() {
		t.Errorf("expected zero time for unknown account, got %v", got)
	}
}

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")

	s1, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := time.Date(2026, 4, 11, 22, 0, 0, 0, time.UTC)
	if err := s1.SetLastSync("primary", want); err != nil {
		t.Fatalf("SetLastSync: %v", err)
	}

	s2, err := Load(p)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got := s2.LastSync("primary")
	if !got.Equal(want) {
		t.Errorf("round-trip: got %v, want %v", got, want)
	}
	if !s2.LastSync("nope").IsZero() {
		t.Error("unrelated account should be zero")
	}
}

func TestConcurrentSetLastSync(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	s, _ := Load(p)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = s.SetLastSync("acct", time.Now())
		}(i)
	}
	wg.Wait()

	reloaded, err := Load(p)
	if err != nil {
		t.Fatalf("reload after concurrent writes: %v", err)
	}
	if reloaded.LastSync("acct").IsZero() {
		t.Error("expected at least one write to land")
	}
}

func TestDefaultPathFor(t *testing.T) {
	if got := DefaultPathFor("/etc/sgbot/config.yaml"); got != "/etc/sgbot/state.json" {
		t.Errorf("got %q", got)
	}
	if got := DefaultPathFor(""); got != "state.json" {
		t.Errorf("empty: got %q", got)
	}
}
