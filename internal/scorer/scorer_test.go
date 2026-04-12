package scorer

import (
	"testing"
	"time"

	sg "github.com/heinrichb/steamgifts-bot/internal/steamgifts"
)

func TestRankPrefersSnipeable(t *testing.T) {
	now := time.Now()
	giveaways := []sg.Giveaway{
		{Code: "expensive", Name: "Expensive Far Out", Cost: 50, Entries: 500, Copies: 1,
			EndsAt: now.Add(24 * time.Hour)},
		{Code: "sniper", Name: "Closing Soon Low Entries", Cost: 5, Entries: 3, Copies: 1,
			EndsAt: now.Add(30 * time.Minute)},
		{Code: "cheap", Name: "Cheap Far Out", Cost: 1, Entries: 100, Copies: 1,
			EndsAt: now.Add(48 * time.Hour)},
	}
	ranked := Rank(giveaways, Context{Weights: Weights{}.WithDefaults()})
	if ranked[0].Code != "sniper" {
		t.Errorf("expected sniper first, got %q (score %.2f)", ranked[0].Code, ranked[0].Score)
	}
}

func TestSniperScoreZeroWhenFarOut(t *testing.T) {
	g := sg.Giveaway{Cost: 10, Entries: 5, Copies: 1,
		EndsAt: time.Now().Add(72 * time.Hour)}
	if s := sniperScore(g, time.Now(), DefaultSniperWeight, DefaultSniperHours); s != 0 {
		t.Errorf("expected 0 sniper score for far-out giveaway, got %.2f", s)
	}
}

func TestSniperScoreHighWhenClosingWithFewEntries(t *testing.T) {
	now := time.Now()
	g := sg.Giveaway{Cost: 5, Entries: 1, Copies: 1,
		EndsAt: now.Add(10 * time.Minute)}
	if s := sniperScore(g, now, DefaultSniperWeight, DefaultSniperHours); s < 5.0 {
		t.Errorf("expected high sniper score, got %.2f", s)
	}
}

func TestCostScorePrefersCheap(t *testing.T) {
	cheap := costScore(sg.Giveaway{Cost: 1}, DefaultCostWeight)
	expensive := costScore(sg.Giveaway{Cost: 50}, DefaultCostWeight)
	if cheap <= expensive {
		t.Errorf("cheap (%.2f) should beat expensive (%.2f)", cheap, expensive)
	}
}

func TestRankStableOnTiedScores(t *testing.T) {
	g := sg.Giveaway{Code: "A", Cost: 25, Entries: 100, Copies: 1,
		EndsAt: time.Now().Add(24 * time.Hour)}
	ranked := Rank([]sg.Giveaway{g, g, g}, Context{})
	if len(ranked) != 3 {
		t.Fatalf("expected 3, got %d", len(ranked))
	}
}

func TestWishlistBoostOutranksNonWishlist(t *testing.T) {
	now := time.Now()
	giveaways := []sg.Giveaway{
		{Code: "cheap", Name: "Cheap Random", Cost: 1, Entries: 100, Copies: 1,
			EndsAt: now.Add(24 * time.Hour)},
		{Code: "wanted", Name: "Wishlist Game", Cost: 30, Entries: 100, Copies: 1,
			EndsAt: now.Add(24 * time.Hour)},
	}
	ctx := Context{
		WishlistCodes: map[string]bool{"wanted": true},
		Weights:       Weights{}.WithDefaults(),
	}
	ranked := Rank(giveaways, ctx)
	if ranked[0].Code != "wanted" {
		t.Errorf("wishlist game should rank first, got %q (score %.2f)", ranked[0].Code, ranked[0].Score)
	}
}

func TestLevelLockedBoost(t *testing.T) {
	now := time.Now()
	giveaways := []sg.Giveaway{
		{Code: "open", Name: "No Level Req", Cost: 5, Level: 0, Entries: 50, Copies: 1,
			EndsAt: now.Add(24 * time.Hour)},
		{Code: "locked", Name: "Level 8 Locked", Cost: 5, Level: 8, Entries: 50, Copies: 1,
			EndsAt: now.Add(24 * time.Hour)},
	}
	ctx := Context{AccountLevel: 10, Weights: Weights{}.WithDefaults()}
	ranked := Rank(giveaways, ctx)
	if ranked[0].Code != "locked" {
		t.Errorf("level-locked game should rank first, got %q", ranked[0].Code)
	}
}

func TestLevelScoreZeroWhenUnknown(t *testing.T) {
	if s := levelScore(sg.Giveaway{Level: 5}, 0, DefaultLevelWeight); s != 0 {
		t.Errorf("expected 0 with unknown account level, got %.2f", s)
	}
	if s := levelScore(sg.Giveaway{Level: 0}, 5, DefaultLevelWeight); s != 0 {
		t.Errorf("expected 0 with no level requirement, got %.2f", s)
	}
}

func TestCustomWeightsOverrideDefaults(t *testing.T) {
	now := time.Now()
	giveaways := []sg.Giveaway{
		{Code: "cheap", Name: "Cheap", Cost: 1, Entries: 100, Copies: 1,
			EndsAt: now.Add(24 * time.Hour)},
		{Code: "wanted", Name: "Wishlist", Cost: 30, Entries: 100, Copies: 1,
			EndsAt: now.Add(24 * time.Hour)},
	}
	// With a very high wishlist weight, the expensive wishlist game should dominate
	ctx := Context{
		WishlistCodes: map[string]bool{"wanted": true},
		Weights:       Weights{Wishlist: 100},
	}
	ranked := Rank(giveaways, ctx)
	if ranked[0].Code != "wanted" || ranked[0].Score < 100 {
		t.Errorf("custom wishlist weight should dominate, got %q (%.2f)", ranked[0].Code, ranked[0].Score)
	}
}

func TestRankEmptyInput(t *testing.T) {
	ranked := Rank(nil, Context{Weights: Weights{}.WithDefaults()})
	if len(ranked) != 0 {
		t.Errorf("expected empty, got %d", len(ranked))
	}
}
