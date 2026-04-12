package account

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/heinrichb/steamgifts-bot/internal/client"
	"github.com/heinrichb/steamgifts-bot/internal/config"
	"github.com/heinrichb/steamgifts-bot/internal/ratelimit"
	sg "github.com/heinrichb/steamgifts-bot/internal/steamgifts"
)

const testPage = `<!doctype html>
<html><body>
<a class="nav__avatar-outer-wrap" href="/user/testuser"></a>
<a class="nav__button-container">
  <span class="nav__points">200</span>P / <span title="5.00">Level 5</span>
</a>
<input type="hidden" name="xsrf_token" value="tok123" />
<div class="giveaway__row-outer-wrap"><div class="giveaway__row-inner-wrap">
  <div class="giveaway__heading">
    <a class="giveaway__heading__name" href="/giveaway/AAA1/game-a">Game A</a>
    <span class="giveaway__heading__thin">(10P)</span>
  </div>
  <div class="giveaway__columns">
    <span data-timestamp="9999999999">5 days</span>
    <div class="giveaway__column--contributor-level giveaway__column--contributor-level--positive">Level 1+</div>
  </div>
  <div class="giveaway__links"><a href="/giveaway/AAA1/game-a/entries">50 entries</a></div>
</div></div>
<div class="giveaway__row-outer-wrap"><div class="giveaway__row-inner-wrap">
  <div class="giveaway__heading">
    <a class="giveaway__heading__name" href="/giveaway/BBB2/game-b">Game B</a>
    <span class="giveaway__heading__thin">(5P)</span>
  </div>
  <div class="giveaway__columns">
    <span data-timestamp="9999999999">5 days</span>
  </div>
  <div class="giveaway__links"><a href="/giveaway/BBB2/game-b/entries">20 entries</a></div>
</div></div>
<div class="giveaway__row-outer-wrap"><div class="giveaway__row-inner-wrap">
  <div class="giveaway__heading">
    <a class="giveaway__heading__name" href="/giveaway/CCC3/game-c">Game C</a>
    <span class="giveaway__heading__thin">(3P)</span>
  </div>
  <div class="giveaway__columns">
    <span data-timestamp="9999999999">5 days</span>
    <div class="giveaway__column--contributor-level giveaway__column--contributor-level--negative">Level 8+</div>
  </div>
  <div class="giveaway__links"><a href="/giveaway/CCC3/game-c/entries">2 entries</a></div>
</div></div>
</body></html>`

func newTestRunner(t *testing.T, srv *httptest.Server, opts func(*Runner)) *Runner {
	t.Helper()
	minPts := 0
	pause := 1
	pinned := false
	maxEntries := 25
	maxPages := 1
	syncEnabled := false
	c, err := client.New("test-cookie", "test-ua",
		client.WithBaseURL(srv.URL),
		client.WithLimiter(ratelimit.New(0, 0)),
		client.WithTimeout(2*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	r := &Runner{
		Name: "test",
		Settings: config.AccountSettings{
			MinPoints:        &minPts,
			PauseMinutes:     &pause,
			EnterPinned:      &pinned,
			MaxEntriesPerRun: &maxEntries,
			MaxPages:         &maxPages,
			SteamSyncEnabled: &syncEnabled,
			Filters:          []string{sg.FilterAll},
		},
		Client: c,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		DryRun: true,
	}
	if opts != nil {
		opts(r)
	}
	return r
}

func TestRunOnceDryRunEntersGiveaways(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, testPage)
	}))
	defer srv.Close()

	r := newTestRunner(t, srv, nil)
	if err := r.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce: %v", err)
	}
	snap := r.Snapshot()
	// Game A (10P) and Game B (5P) are joinable; Game C requires level 8 (account is 5)
	if snap.EntriesOK != 2 {
		t.Errorf("expected 2 entries, got %d", snap.EntriesOK)
	}
}

func TestRunOnceRespectsMaxEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, testPage)
	}))
	defer srv.Close()

	maxEntries := 1
	r := newTestRunner(t, srv, func(r *Runner) {
		r.Settings.MaxEntriesPerRun = &maxEntries
	})
	if err := r.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce: %v", err)
	}
	if snap := r.Snapshot(); snap.EntriesOK != 1 {
		t.Errorf("expected 1 entry (max_entries cap), got %d", snap.EntriesOK)
	}
}

func TestRunOnceSkipsLevelLocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, testPage)
	}))
	defer srv.Close()

	r := newTestRunner(t, srv, nil)
	_ = r.runOnce(context.Background())
	snap := r.Snapshot()
	for _, e := range snap.RecentEntries {
		if e.Code == "CCC3" {
			t.Error("should not have entered level-8-locked Game C at account level 5")
		}
	}
}

func TestRunOnceDeduplicatesAcrossPages(t *testing.T) {
	// Serve the same page for both page=1 and page=2
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, testPage)
	}))
	defer srv.Close()

	maxPages := 2
	r := newTestRunner(t, srv, func(r *Runner) {
		r.Settings.MaxPages = &maxPages
	})
	_ = r.runOnce(context.Background())
	snap := r.Snapshot()
	// Same giveaways on both pages — should still only enter each once
	if snap.EntriesOK != 2 {
		t.Errorf("expected 2 entries (deduped across pages), got %d", snap.EntriesOK)
	}
}

func TestRunOnceRealEntryPostsCorrectly(t *testing.T) {
	var postedCodes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.ParseForm()
			postedCodes = append(postedCodes, r.PostForm.Get("code"))
			io.WriteString(w, `{"type":"success","entry_count":"1","points":190}`)
			return
		}
		io.WriteString(w, testPage)
	}))
	defer srv.Close()

	r := newTestRunner(t, srv, func(r *Runner) { r.DryRun = false })
	if err := r.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce: %v", err)
	}
	if len(postedCodes) != 2 {
		t.Fatalf("expected 2 POSTs, got %d: %v", len(postedCodes), postedCodes)
	}
	// Should not have posted CCC3 (level-locked)
	for _, code := range postedCodes {
		if code == "CCC3" {
			t.Error("should not POST level-locked giveaway")
		}
	}
}

func TestRunOnceHandlesEntryErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			io.WriteString(w, `{"type":"error","msg":"Level 2 Required","points":"200"}`)
			return
		}
		io.WriteString(w, testPage)
	}))
	defer srv.Close()

	r := newTestRunner(t, srv, func(r *Runner) { r.DryRun = false })
	err := r.runOnce(context.Background())
	if err != nil {
		t.Fatalf("runOnce should not fail on entry errors: %v", err)
	}
	snap := r.Snapshot()
	if snap.EntriesOK != 0 {
		t.Errorf("no entries should succeed, got %d OK", snap.EntriesOK)
	}
	if snap.EntriesAttempt != 2 {
		t.Errorf("expected 2 attempts, got %d", snap.EntriesAttempt)
	}
}

func TestRunOnceContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, testPage)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	r := newTestRunner(t, srv, nil)
	err := r.runOnce(ctx)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected context canceled error, got: %v", err)
	}
}
