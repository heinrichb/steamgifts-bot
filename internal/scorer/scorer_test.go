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

	ranked := Rank(giveaways, Context{})
	if len(ranked) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(ranked))
	}
	if ranked[0].Code != "sniper" {
		t.Errorf("expected sniper first, got %q (score %.2f)", ranked[0].Code, ranked[0].Score)
	}
	if ranked[0].Score <= ranked[1].Score {
		t.Errorf("sniper score (%.2f) should beat %q (%.2f)",
			ranked[0].Score, ranked[1].Code, ranked[1].Score)
	}
}

func TestSniperScoreZeroWhenFarOut(t *testing.T) {
	g := sg.Giveaway{Cost: 10, Entries: 5, Copies: 1,
		EndsAt: time.Now().Add(72 * time.Hour)}
	s := sniperScore(g, time.Now())
	if s != 0 {
		t.Errorf("expected 0 sniper score for far-out giveaway, got %.2f", s)
	}
}

func TestSniperScoreHighWhenClosingWithFewEntries(t *testing.T) {
	now := time.Now()
	g := sg.Giveaway{Cost: 5, Entries: 1, Copies: 1,
		EndsAt: now.Add(10 * time.Minute)}
	s := sniperScore(g, now)
	if s < 5.0 {
		t.Errorf("expected high sniper score, got %.2f", s)
	}
}

func TestCostScorePrefersChp(t *testing.T) {
	cheap := costScore(sg.Giveaway{Cost: 1})
	expensive := costScore(sg.Giveaway{Cost: 50})
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
	ctx := Context{WishlistCodes: map[string]bool{"wanted": true}}
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
	ctx := Context{AccountLevel: 10}
	ranked := Rank(giveaways, ctx)
	if ranked[0].Code != "locked" {
		t.Errorf("level-locked game should rank first, got %q (score %.2f vs %.2f)",
			ranked[0].Code, ranked[0].Score, ranked[1].Score)
	}
}

func TestLevelScoreZeroWhenUnknown(t *testing.T) {
	g := sg.Giveaway{Level: 5}
	if s := levelScore(g, 0); s != 0 {
		t.Errorf("expected 0 with unknown account level, got %.2f", s)
	}
	g2 := sg.Giveaway{Level: 0}
	if s := levelScore(g2, 5); s != 0 {
		t.Errorf("expected 0 with no level requirement, got %.2f", s)
	}
}

func TestRankEmptyInput(t *testing.T) {
	ranked := Rank(nil, Context{})
	if len(ranked) != 0 {
		t.Errorf("expected empty, got %d", len(ranked))
	}
}
