package cli

import (
	"testing"
	"time"
)

func TestRedact(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Short", "abc", "********"},
		{"ExactlyEight", "12345678", "********"},
		{"Longer", "abcdefghij", "abcd…ghij"},
		{"Typical", "a1b2c3d4e5f6g7h8", "a1b2…g7h8"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := redact(tc.input)
			if got != tc.expected {
				t.Errorf("redact(%q) = %q; want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestHumanizeUntilFuture(t *testing.T) {
	future := time.Now().Add(5 * time.Minute)
	got := humanizeUntil(future)
	if got == "now" {
		t.Error("expected future time not to be 'now'")
	}
	if got[:3] != "in " {
		t.Errorf("expected 'in ...' prefix, got %q", got)
	}
}

func TestHumanizeUntilPast(t *testing.T) {
	past := time.Now().Add(-1 * time.Minute)
	got := humanizeUntil(past)
	if got != "now" {
		t.Errorf("expected 'now' for past time, got %q", got)
	}
}

func TestHumanizeAgo(t *testing.T) {
	past := time.Now().Add(-30 * time.Second)
	got := humanizeAgo(past)
	if got == "" {
		t.Error("expected non-empty result")
	}
	if got[len(got)-4:] != " ago" {
		t.Errorf("expected ' ago' suffix, got %q", got)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		n        int
		expected string
	}{
		{"Short", "hi", 10, "hi"},
		{"ExactLen", "hello", 5, "hello"},
		{"NeedsTruncation", "hello world", 5, "hell…"},
		{"Unicode", "héllo wörld", 6, "héllo…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.input, tc.n)
			if got != tc.expected {
				t.Errorf("truncate(%q, %d) = %q; want %q", tc.input, tc.n, got, tc.expected)
			}
		})
	}
}

func TestOrDash(t *testing.T) {
	if got := orDash(""); got != "-" {
		t.Errorf("orDash('') = %q; want '-'", got)
	}
	if got := orDash("hello"); got != "hello" {
		t.Errorf("orDash('hello') = %q; want 'hello'", got)
	}
}
