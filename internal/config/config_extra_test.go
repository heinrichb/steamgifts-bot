package config

import (
	"strings"
	"testing"
	"time"
)

func TestAccountSettingsNilDefaults(t *testing.T) {
	// All nil pointer fields should return their documented defaults.
	s := AccountSettings{}
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"MaxPagesValue", s.MaxPagesValue(), 1},
		{"MaxEntriesPerAppValue", s.MaxEntriesPerAppValue(), 0},
		{"SteamSyncEnabledValue", s.SteamSyncEnabledValue(), false},
		{"SteamSyncInterval", s.SteamSyncInterval(), 24 * time.Hour},
		{"PauseDuration", s.PauseDuration(), 15 * time.Minute},
		{"MinPointsValue", s.MinPointsValue(), 0},
		{"MaxEntriesValue", s.MaxEntriesValue(), 0},
		{"EnterPinnedValue", s.EnterPinnedValue(), false},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s nil: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestAccountSettingsSetValues(t *testing.T) {
	five := 5
	three := 3
	twelve := 12
	thirty := 30
	hundred := 100
	ten := 10
	tr := true
	s := AccountSettings{
		MaxPages:               &five,
		MaxEntriesPerApp:       &three,
		SteamSyncEnabled:       &tr,
		SteamSyncIntervalHours: &twelve,
		PauseMinutes:           &thirty,
		MinPoints:              &hundred,
		MaxEntriesPerRun:       &ten,
		EnterPinned:            &tr,
	}
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"MaxPagesValue", s.MaxPagesValue(), 5},
		{"MaxEntriesPerAppValue", s.MaxEntriesPerAppValue(), 3},
		{"SteamSyncEnabledValue", s.SteamSyncEnabledValue(), true},
		{"SteamSyncInterval", s.SteamSyncInterval(), 12 * time.Hour},
		{"PauseDuration", s.PauseDuration(), 30 * time.Minute},
		{"MinPointsValue", s.MinPointsValue(), 100},
		{"MaxEntriesValue", s.MaxEntriesValue(), 10},
		{"EnterPinnedValue", s.EnterPinnedValue(), true},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s set: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestMaxPagesValueClampsZeroToOne(t *testing.T) {
	zero := 0
	s := AccountSettings{MaxPages: &zero}
	if got := s.MaxPagesValue(); got != 1 {
		t.Errorf("MaxPagesValue(0) = %d; want 1 (clamped)", got)
	}
}

func TestResolvedOutOfBounds(t *testing.T) {
	c := Defaults()
	// Negative and beyond-length indexes should return global defaults.
	for _, idx := range []int{-1, 100} {
		got := c.Resolved(idx)
		if got.MinPointsValue() != 50 {
			t.Errorf("Resolved(%d) should return defaults", idx)
		}
	}
}

func TestResolvedMergesAllOverrides(t *testing.T) {
	c := Defaults()
	mp := 200
	pause := 30
	pinned := true
	hours := 48
	syncOff := false
	perApp := 2
	pages := 10
	entries := 5
	c.Accounts = []Account{{
		Name:   "alt",
		Cookie: "a",
		AccountSettings: AccountSettings{
			MinPoints:              &mp,
			PauseMinutes:           &pause,
			EnterPinned:            &pinned,
			MaxEntriesPerRun:       &entries,
			UserAgent:              "custom-ua",
			Filters:                []string{"wishlist"},
			MaxPages:               &pages,
			MaxEntriesPerApp:       &perApp,
			ProxyURL:               "socks5://localhost:1080",
			SteamSyncEnabled:       &syncOff,
			SteamSyncIntervalHours: &hours,
		},
	}}
	r := c.Resolved(0)
	if r.MinPointsValue() != 200 {
		t.Errorf("MinPoints: %d", r.MinPointsValue())
	}
	if r.PauseDuration() != 30*time.Minute {
		t.Errorf("Pause: %v", r.PauseDuration())
	}
	if !r.EnterPinnedValue() {
		t.Error("EnterPinned not overridden")
	}
	if r.MaxEntriesValue() != 5 {
		t.Errorf("MaxEntries: %d", r.MaxEntriesValue())
	}
	if r.UserAgent != "custom-ua" {
		t.Errorf("UserAgent: %s", r.UserAgent)
	}
	if len(r.Filters) != 1 || r.Filters[0] != "wishlist" {
		t.Errorf("Filters: %v", r.Filters)
	}
	if r.MaxPagesValue() != 10 {
		t.Errorf("MaxPages: %d", r.MaxPagesValue())
	}
	if r.MaxEntriesPerAppValue() != 2 {
		t.Errorf("MaxEntriesPerApp: %d", r.MaxEntriesPerAppValue())
	}
	if r.ProxyURL != "socks5://localhost:1080" {
		t.Errorf("ProxyURL: %s", r.ProxyURL)
	}
	if r.SteamSyncEnabledValue() {
		t.Error("SteamSync should be disabled")
	}
	if r.SteamSyncInterval() != 48*time.Hour {
		t.Errorf("SteamSyncInterval: %v", r.SteamSyncInterval())
	}
}

func TestResolvedFiltersFromTopLevel(t *testing.T) {
	c := Defaults()
	c.Defaults.Filters = nil
	c.Accounts = []Account{{Name: "x", Cookie: "a"}}
	got := c.Resolved(0)
	if len(got.Filters) != len(c.Filters) {
		t.Errorf("expected top-level filters fallback, got %v", got.Filters)
	}
}

func TestValidateAllErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		cfg     func() Config
		wantErr string
	}{
		{"EmptyName", func() Config {
			c := Defaults()
			c.Accounts = []Account{{Name: "", Cookie: "abc"}}
			return c
		}, "name is required"},
		{"EmptyCookie", func() Config {
			c := Defaults()
			c.Accounts = []Account{{Name: "x", Cookie: "  "}}
			return c
		}, "cookie is empty"},
		{"MinPointsOutOfRange", func() Config {
			c := Defaults()
			mp := 500
			c.Accounts = []Account{{Name: "x", Cookie: "abc",
				AccountSettings: AccountSettings{MinPoints: &mp}}}
			return c
		}, "min_points"},
		{"PauseTooShort", func() Config {
			c := Defaults()
			p := 0
			c.Accounts = []Account{{Name: "x", Cookie: "abc",
				AccountSettings: AccountSettings{PauseMinutes: &p}}}
			return c
		}, "pause_minutes"},
		{"MaxEntriesNegative", func() Config {
			c := Defaults()
			neg := -1
			c.Accounts = []Account{{Name: "x", Cookie: "abc",
				AccountSettings: AccountSettings{MaxEntriesPerRun: &neg}}}
			return c
		}, "max_entries_per_run"},
		{"SyncIntervalTooLow", func() Config {
			c := Defaults()
			h := 0
			c.Accounts = []Account{{Name: "x", Cookie: "abc",
				AccountSettings: AccountSettings{SteamSyncIntervalHours: &h}}}
			return c
		}, "steam_sync_interval_hours"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg()
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}
