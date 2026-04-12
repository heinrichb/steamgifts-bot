package config

import (
	"strings"
	"testing"
	"time"
)

func intPtr(v int) *int    { return &v }
func boolPtr(v bool) *bool { return &v }

func TestDefaultsAreValidExceptForMissingAccount(t *testing.T) {
	c := Defaults()
	if err := c.Validate(); err == nil {
		t.Fatal("expected default config (no accounts) to fail validation")
	} else if !strings.Contains(err.Error(), "no accounts") {
		t.Fatalf("expected 'no accounts' error, got: %v", err)
	}
}

func TestValidateHappyPath(t *testing.T) {
	c := Defaults()
	c.Accounts = []Account{{Name: "primary", Cookie: "abcd1234"}}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateRejectsPlaceholderCookie(t *testing.T) {
	c := Defaults()
	c.Accounts = []Account{{Name: "primary", Cookie: "REPLACE_WITH_YOUR_PHPSESSID"}}
	if err := c.Validate(); err == nil {
		t.Fatal("expected placeholder cookie to fail validation")
	}
}

func TestValidateRejectsDuplicateNames(t *testing.T) {
	c := Defaults()
	c.Accounts = []Account{
		{Name: "x", Cookie: "a"},
		{Name: "x", Cookie: "b"},
	}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate-name error, got: %v", err)
	}
}

func TestValidateRejectsUnknownFilter(t *testing.T) {
	c := Defaults()
	c.Filters = []string{"wishlist", "bogus"}
	c.Accounts = []Account{{Name: "x", Cookie: "a"}}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "unknown filter") {
		t.Fatalf("expected unknown-filter error, got: %v", err)
	}
}

func TestResolvedAppliesOverrides(t *testing.T) {
	c := Defaults()
	c.Accounts = []Account{
		{
			Name:   "alt",
			Cookie: "a",
			AccountSettings: AccountSettings{
				MinPoints:    intPtr(200),
				PauseMinutes: intPtr(30),
				EnterPinned:  boolPtr(true),
				Filters:      []string{"wishlist"},
			},
		},
	}
	r := c.Resolved(0)
	if got := r.MinPointsValue(); got != 200 {
		t.Errorf("MinPoints override: got %d, want 200", got)
	}
	if got := r.PauseDuration(); got != 30*time.Minute {
		t.Errorf("PauseDuration override: got %s, want 30m", got)
	}
	if !r.EnterPinnedValue() {
		t.Error("EnterPinned override not applied")
	}
	if len(r.Filters) != 1 || r.Filters[0] != "wishlist" {
		t.Errorf("Filters override: got %v", r.Filters)
	}
}

func TestResolvedFallsBackToDefaults(t *testing.T) {
	c := Defaults()
	c.Accounts = []Account{{Name: "x", Cookie: "a"}}
	r := c.Resolved(0)
	if r.MinPointsValue() != 50 {
		t.Errorf("default MinPoints: got %d, want 50", r.MinPointsValue())
	}
	if r.PauseDuration() != 15*time.Minute {
		t.Errorf("default Pause: got %s, want 15m", r.PauseDuration())
	}
	if r.EnterPinnedValue() {
		t.Error("default EnterPinned should be false")
	}
	if len(r.Filters) != len(c.Filters) {
		t.Errorf("expected default filters to fall back to global list (%d), got %d: %v",
			len(c.Filters), len(r.Filters), r.Filters)
	}
}
