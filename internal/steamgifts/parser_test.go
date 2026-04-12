package steamgifts

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestParseListPageBasic(t *testing.T) {
	state, gs, err := ParseListPage(loadFixture(t, "list_basic.html"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if state.Username != "testuser" {
		t.Errorf("username: got %q, want testuser", state.Username)
	}
	if state.Points != 137 {
		t.Errorf("points: got %d, want 137", state.Points)
	}
	if state.XSRFToken != "abc123token" {
		t.Errorf("xsrf: got %q", state.XSRFToken)
	}
	if len(gs) != 4 {
		t.Fatalf("expected 4 giveaways, got %d: %+v", len(gs), gs)
	}

	byCode := map[string]Giveaway{}
	for _, g := range gs {
		byCode[g.Code] = g
	}

	super, ok := byCode["AAAA1"]
	if !ok {
		t.Fatal("missing AAAA1")
	}
	if super.Name != "Super Game" || super.Cost != 50 || super.Copies != 3 || super.Entries != 42 {
		t.Errorf("super game parsed wrong: %+v", super)
	}
	if super.Pinned || super.Entered || super.Unjoinable {
		t.Errorf("super game flags wrong: %+v", super)
	}

	entered, ok := byCode["BBBB2"]
	if !ok {
		t.Fatal("missing BBBB2")
	}
	if !entered.Entered {
		t.Errorf("BBBB2 should be marked Entered: %+v", entered)
	}

	pinned, ok := byCode["CCCC3"]
	if !ok {
		t.Fatal("missing CCCC3")
	}
	if !pinned.Pinned {
		t.Errorf("CCCC3 should be marked Pinned: %+v", pinned)
	}
	if pinned.Entries != 1234 {
		t.Errorf("CCCC3 entries with comma not parsed: %d", pinned.Entries)
	}
}

func TestJoinableLogic(t *testing.T) {
	_, gs, err := ParseListPage(loadFixture(t, "list_basic.html"))
	if err != nil {
		t.Fatal(err)
	}
	byCode := map[string]Giveaway{}
	for _, g := range gs {
		byCode[g.Code] = g
	}

	// Currently 137 points, min 50 — Super Game costs 50, 137-50=87 >= 50 ✓
	if !byCode["AAAA1"].Joinable(137, 50, false) {
		t.Error("AAAA1 should be joinable with 137pts/min50")
	}
	// Min too high.
	if byCode["AAAA1"].Joinable(137, 100, false) {
		t.Error("AAAA1 should not be joinable when min would be violated")
	}
	// Already-entered.
	if byCode["BBBB2"].Joinable(137, 0, false) {
		t.Error("BBBB2 already entered, must not be joinable")
	}
	// Pinned blocked unless allowed.
	if byCode["CCCC3"].Joinable(137, 0, false) {
		t.Error("CCCC3 pinned should be skipped when allowPinned=false")
	}
	if !byCode["CCCC3"].Joinable(137, 0, true) {
		t.Error("CCCC3 pinned should join when allowPinned=true")
	}
	// Expired.
	if byCode["DDDD4"].Joinable(137, 0, true) {
		t.Error("DDDD4 expired must not be joinable")
	}
}

func TestParseListPageEmpty(t *testing.T) {
	state, gs, err := ParseListPage(loadFixture(t, "list_empty.html"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if state.XSRFToken != "emptytoken" {
		t.Errorf("xsrf: %q", state.XSRFToken)
	}
	if state.Username != "quietuser" {
		t.Errorf("username: %q", state.Username)
	}
	if len(gs) != 0 {
		t.Errorf("expected zero giveaways, got %d", len(gs))
	}
}

func TestParseListPageMissingXSRFFails(t *testing.T) {
	_, _, err := ParseListPage(loadFixture(t, "list_no_xsrf.html"))
	if err == nil {
		t.Fatal("expected error when xsrf missing")
	}
}
