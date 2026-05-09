// Package steamgifts holds the domain types, HTML parsers, and request
// helpers for talking to https://www.steamgifts.com.
package steamgifts

import "time"

// Giveaway represents a single steamgifts.com giveaway parsed from a list page.
//
// Code is the short opaque ID embedded in the giveaway URL
// (e.g. /giveaway/<code>/<slug>) and is what entry POSTs reference.
type Giveaway struct {
	Code       string
	Name       string
	URL        string
	Cost       int       // points required to enter
	Copies     int       // number of copies offered
	Entries    int       // current entry count
	EndsAt     time.Time // when the giveaway closes (zero if unparseable)
	Level      int       // minimum contributor level required (0 = no requirement)
	Pinned     bool
	Entered    bool // user has already entered this giveaway
	Unjoinable bool // faded for any other reason (login required, region locked, etc.)
}

// Joinable reports whether the bot should attempt to enter this giveaway,
// given the resolved settings for the active account.
func (g Giveaway) Joinable(currentPoints, minPoints, accountLevel int, allowPinned bool) bool {
	if g.Entered || g.Unjoinable {
		return false
	}
	// Skip if we know both the requirement and the account level.
	// If accountLevel is 0 (unparsed), let the server reject — saves
	// the bot from accidentally skipping everything when parsing fails.
	if g.Level > 0 && accountLevel > 0 && accountLevel < g.Level {
		return false
	}
	if g.Pinned && !allowPinned {
		return false
	}
	if !g.EndsAt.IsZero() && time.Now().After(g.EndsAt) {
		return false
	}
	if g.Cost < 0 {
		return false
	}
	if g.Cost > 0 && currentPoints-g.Cost < minPoints {
		return false
	}
	return true
}

// AccountState is what we learn about the signed-in account from any
// list page (the site puts these in the header on every authenticated request).
type AccountState struct {
	Username  string
	Points    int
	Level     int
	XSRFToken string
}
