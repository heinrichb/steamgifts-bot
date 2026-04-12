// Package scorer assigns a priority score to each giveaway candidate so
// the bot enters the highest-value ones first. Higher score = enter sooner.
//
// The score is a weighted sum of independent components:
//
//   - Sniper boost: giveaways closing soon with few entries per copy
//     have the best win probability — score increases as deadline
//     approaches and entry count stays low.
//   - Cost efficiency: mild preference for cheaper giveaways so the bot
//     spreads points across more entries rather than blowing them on
//     one expensive title.
//
// Future components (see TODO.md): wishlist boost, level-locked boost,
// popularity/quality boost.
package scorer

import (
	"math"
	"sort"
	"time"

	sg "github.com/heinrichb/steamgifts-bot/internal/steamgifts"
)

const (
	wishlistWeight       = 5.0
	sniperWeight         = 10.0
	sniperThresholdHours = 2.0
	costWeight           = 1.0
	maxCost              = 50.0
)

// Context carries per-cycle data the scorer needs beyond the giveaway itself.
type Context struct {
	WishlistCodes map[string]bool // giveaway codes found on the wishlist filter
}

// Candidate wraps a giveaway with its computed score.
type Candidate struct {
	sg.Giveaway
	Score float64
}

// Rank scores and sorts a slice of giveaways, highest score first.
// The input slice is not modified; a new sorted slice of Candidates
// is returned.
func Rank(giveaways []sg.Giveaway, sctx Context) []Candidate {
	now := time.Now()
	candidates := make([]Candidate, len(giveaways))
	for i, g := range giveaways {
		candidates[i] = Candidate{
			Giveaway: g,
			Score:    score(g, sctx, now),
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	return candidates
}

func score(g sg.Giveaway, sctx Context, now time.Time) float64 {
	s := sniperScore(g, now) + costScore(g)
	if sctx.WishlistCodes[g.Code] {
		s += wishlistWeight
	}
	return s
}

// sniperScore boosts giveaways that are closing soon with few entries
// relative to the number of copies. The closer the deadline and the
// fewer competitors per copy, the higher the expected value.
func sniperScore(g sg.Giveaway, now time.Time) float64 {
	if g.EndsAt.IsZero() || g.EndsAt.Before(now) {
		return 0
	}
	hoursLeft := g.EndsAt.Sub(now).Hours()
	if hoursLeft > sniperThresholdHours {
		return 0
	}
	// urgency: 1.0 at deadline, 0.0 at threshold
	urgency := 1.0 - (hoursLeft / sniperThresholdHours)

	// winRate: copies / max(entries, 1) — capped at 1.0
	entries := float64(g.Entries)
	if entries < 1 {
		entries = 1
	}
	copies := float64(g.Copies)
	if copies < 1 {
		copies = 1
	}
	winRate := math.Min(copies/entries, 1.0)

	return sniperWeight * urgency * winRate
}

// costScore gives a mild preference to cheaper giveaways so the bot
// enters more titles per point budget rather than blowing everything
// on one expensive entry.
func costScore(g sg.Giveaway) float64 {
	cost := float64(g.Cost)
	if cost < 1 {
		cost = 1
	}
	return costWeight * (1.0 - math.Min(cost/maxCost, 1.0))
}
