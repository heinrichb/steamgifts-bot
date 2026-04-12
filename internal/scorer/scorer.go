// Package scorer assigns a priority score to each giveaway candidate so
// the bot enters the highest-value ones first. Higher score = enter sooner.
//
// The score is a weighted sum of independent components: sniper boost,
// wishlist boost, level-locked boost, and cost efficiency. All weights
// are configurable via config.yml's `scorer:` section.
package scorer

import (
	"math"
	"sort"
	"time"

	sg "github.com/heinrichb/steamgifts-bot/internal/steamgifts"
)

// Default weights used when config values are unset.
const (
	DefaultWishlistWeight = 5.0
	DefaultSniperWeight   = 10.0
	DefaultSniperHours    = 2.0
	DefaultLevelWeight    = 3.0
	DefaultCostWeight     = 1.0
)

const levelMaxBoost = 10

// Weights holds the tunable scoring parameters. Zero values use defaults.
type Weights struct {
	Wishlist    float64
	Sniper      float64
	SniperHours float64
	Level       float64
	Cost        float64
}

// DefaultWeights returns a Weights with all built-in defaults populated.
func DefaultWeights() Weights {
	return Weights{
		Wishlist:    DefaultWishlistWeight,
		Sniper:      DefaultSniperWeight,
		SniperHours: DefaultSniperHours,
		Level:       DefaultLevelWeight,
		Cost:        DefaultCostWeight,
	}
}

// Context carries per-cycle data the scorer needs beyond the giveaway itself.
type Context struct {
	WishlistCodes map[string]bool
	AccountLevel  int
	Weights       Weights
}

// Candidate wraps a giveaway with its computed score.
type Candidate struct {
	sg.Giveaway
	Score float64
}

// Rank scores and sorts a slice of giveaways, highest score first.
func Rank(giveaways []sg.Giveaway, sctx Context) []Candidate {
	now := time.Now()
	candidates := make([]Candidate, len(giveaways))
	for i, g := range giveaways {
		candidates[i] = Candidate{
			Giveaway: g,
			Score:    score(g, sctx, now),
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	return candidates
}

func score(g sg.Giveaway, sctx Context, now time.Time) float64 {
	w := sctx.Weights
	s := sniperScore(g, now, w.Sniper, w.SniperHours) +
		valueScore(g, w.Cost) +
		levelScore(g, sctx.AccountLevel, w.Level)
	if sctx.WishlistCodes[g.Code] {
		s += w.Wishlist
	}
	return s
}

// levelScore boosts level-locked giveaways. Higher requirements mean fewer
// eligible users, and that advantage is amplified when entry count is low.
func levelScore(g sg.Giveaway, accountLevel int, weight float64) float64 {
	if g.Level <= 0 || accountLevel <= 0 {
		return 0
	}
	levelFactor := math.Min(float64(g.Level)/float64(levelMaxBoost), 1.0)
	entries := math.Max(float64(g.Entries), 1)
	scarcity := 1.0 / math.Log2(entries+1)
	return weight * levelFactor * scarcity
}

func sniperScore(g sg.Giveaway, now time.Time, weight, thresholdHours float64) float64 {
	if g.EndsAt.IsZero() || g.EndsAt.Before(now) {
		return 0
	}
	hoursLeft := g.EndsAt.Sub(now).Hours()
	if hoursLeft > thresholdHours {
		return 0
	}
	urgency := 1.0 - (hoursLeft / thresholdHours)
	entries := math.Max(float64(g.Entries), 1)
	copies := math.Max(float64(g.Copies), 1)
	winRate := math.Min(copies/entries, 1.0)
	return weight * urgency * winRate
}

// valueScore rewards expected value: win probability per point spent.
// Formula: weight * log1p((copies / entries / cost) * 100).
// The log scale compresses the range so high-EV outliers don't dominate.
func valueScore(g sg.Giveaway, weight float64) float64 {
	entries := math.Max(float64(g.Entries), 1)
	copies := math.Max(float64(g.Copies), 1)
	cost := math.Max(float64(g.Cost), 1)
	ev := (copies / entries) / cost
	// Log scale: raw EV can span many orders of magnitude.
	// log1p maps [0, inf) → [0, inf) with diminishing returns.
	return weight * math.Log1p(ev*100)
}
