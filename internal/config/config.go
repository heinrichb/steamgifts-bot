// Package config defines the user-facing configuration schema for steamgifts-bot.
//
// The schema is loaded with this precedence (highest first):
//
//  1. CLI flags
//  2. Environment variables (STEAMGIFTS_*)
//  3. config.yaml (or any path passed via --config)
//  4. Built-in Defaults() values
//
// Loading and merging from disk lives in cli.loadConfig — this package owns
// only the schema, defaults, validation, and pure helpers so it stays
// importable from tests and tools without dragging in viper.
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Filter names accepted in the YAML `filters` list.
const (
	FilterWishlist    = "wishlist"
	FilterGroup       = "group"
	FilterRecommended = "recommended"
	FilterNew         = "new"
	FilterAll         = "all"
)

// ValidFilters is the canonical set of filter identifiers.
var ValidFilters = []string{
	FilterWishlist,
	FilterGroup,
	FilterRecommended,
	FilterNew,
	FilterAll,
}

// Config is the root user-facing configuration object.
type Config struct {
	Defaults AccountSettings `yaml:"defaults"  mapstructure:"defaults"`
	Filters  []string        `yaml:"filters"   mapstructure:"filters"`
	Accounts []Account       `yaml:"accounts"  mapstructure:"accounts"`
}

// AccountSettings holds the per-account knobs that may be set globally
// (under `defaults`) or overridden inside an individual account entry.
//
// Pointer-typed fields signal "not set" so per-account overrides can
// distinguish "use default" from "explicitly zero".
type AccountSettings struct {
	MinPoints        *int     `yaml:"min_points,omitempty"          mapstructure:"min_points"`
	PauseMinutes     *int     `yaml:"pause_minutes,omitempty"       mapstructure:"pause_minutes"`
	EnterPinned      *bool    `yaml:"enter_pinned,omitempty"        mapstructure:"enter_pinned"`
	MaxEntriesPerRun *int     `yaml:"max_entries_per_run,omitempty" mapstructure:"max_entries_per_run"`
	UserAgent        string   `yaml:"user_agent,omitempty"          mapstructure:"user_agent"`
	Filters          []string `yaml:"filters,omitempty"             mapstructure:"filters"`
}

// Account is a single steamgifts.com identity the bot will operate on.
type Account struct {
	Name            string `yaml:"name"   mapstructure:"name"`
	Cookie          string `yaml:"cookie" mapstructure:"cookie"`
	AccountSettings `yaml:",inline" mapstructure:",squash"`
}

// Defaults returns a Config populated with sensible out-of-the-box values.
// Wizard, validation, and merging all start from this.
func Defaults() Config {
	minPoints := 50
	pause := 15
	pinned := false
	maxEntries := 25
	return Config{
		Defaults: AccountSettings{
			MinPoints:        &minPoints,
			PauseMinutes:     &pause,
			EnterPinned:      &pinned,
			MaxEntriesPerRun: &maxEntries,
			UserAgent:        "steamgifts-bot/0.1 (+https://github.com/heinrichb/steamgifts-bot)",
		},
		Filters: []string{
			FilterWishlist, FilterGroup, FilterRecommended, FilterNew, FilterAll,
		},
		Accounts: nil,
	}
}

// Resolved returns the effective settings for a single account by merging
// the account's overrides on top of the global defaults.
func (c *Config) Resolved(idx int) AccountSettings {
	if idx < 0 || idx >= len(c.Accounts) {
		return c.Defaults
	}
	a := c.Accounts[idx]
	out := c.Defaults
	if a.MinPoints != nil {
		out.MinPoints = a.MinPoints
	}
	if a.PauseMinutes != nil {
		out.PauseMinutes = a.PauseMinutes
	}
	if a.EnterPinned != nil {
		out.EnterPinned = a.EnterPinned
	}
	if a.MaxEntriesPerRun != nil {
		out.MaxEntriesPerRun = a.MaxEntriesPerRun
	}
	if a.UserAgent != "" {
		out.UserAgent = a.UserAgent
	}
	if len(a.Filters) > 0 {
		out.Filters = a.Filters
	} else if len(out.Filters) == 0 {
		out.Filters = c.Filters
	}
	return out
}

// PauseDuration returns the resolved pause as a time.Duration.
func (s AccountSettings) PauseDuration() time.Duration {
	if s.PauseMinutes == nil {
		return 15 * time.Minute
	}
	return time.Duration(*s.PauseMinutes) * time.Minute
}

// MinPointsValue returns the resolved minimum-points threshold.
func (s AccountSettings) MinPointsValue() int {
	if s.MinPoints == nil {
		return 0
	}
	return *s.MinPoints
}

// MaxEntriesValue returns the resolved per-run entry cap.
func (s AccountSettings) MaxEntriesValue() int {
	if s.MaxEntriesPerRun == nil {
		return 0
	}
	return *s.MaxEntriesPerRun
}

// EnterPinnedValue returns whether pinned giveaways should be entered.
func (s AccountSettings) EnterPinnedValue() bool {
	return s.EnterPinned != nil && *s.EnterPinned
}

// Validate checks the config for problems that would prevent the bot
// from running. It returns the first fatal error encountered.
func (c *Config) Validate() error {
	if len(c.Accounts) == 0 {
		return errors.New("no accounts configured: add at least one account or run `steamgifts-bot setup`")
	}

	seen := make(map[string]struct{}, len(c.Accounts))
	for i, a := range c.Accounts {
		name := strings.TrimSpace(a.Name)
		if name == "" {
			return fmt.Errorf("accounts[%d]: name is required", i)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("accounts[%d]: duplicate account name %q", i, name)
		}
		seen[name] = struct{}{}

		if strings.TrimSpace(a.Cookie) == "" || a.Cookie == "REPLACE_WITH_YOUR_PHPSESSID" {
			return fmt.Errorf("accounts[%d] (%s): cookie is empty or unset — run `steamgifts-bot setup` to capture it", i, name)
		}

		resolved := c.Resolved(i)
		if mp := resolved.MinPointsValue(); mp < 0 || mp > 400 {
			return fmt.Errorf("accounts[%d] (%s): min_points %d out of range [0,400]", i, name, mp)
		}
		if pause := resolved.PauseDuration(); pause < time.Minute {
			return fmt.Errorf("accounts[%d] (%s): pause_minutes must be >= 1", i, name)
		}
		if max := resolved.MaxEntriesValue(); max < 0 {
			return fmt.Errorf("accounts[%d] (%s): max_entries_per_run must be >= 0", i, name)
		}
		for _, f := range resolved.Filters {
			if !isValidFilter(f) {
				return fmt.Errorf("accounts[%d] (%s): unknown filter %q (valid: %s)",
					i, name, f, strings.Join(ValidFilters, ", "))
			}
		}
	}
	return nil
}

func isValidFilter(name string) bool {
	for _, v := range ValidFilters {
		if v == name {
			return true
		}
	}
	return false
}
