// Package account drives a single steamgifts.com account: it polls the
// configured filter pages on a schedule and enters joinable giveaways
// until points are exhausted or the per-run cap is reached.
package account

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/heinrichb/steamgifts-bot/internal/client"
	"github.com/heinrichb/steamgifts-bot/internal/config"
	"github.com/heinrichb/steamgifts-bot/internal/metrics"
	"github.com/heinrichb/steamgifts-bot/internal/notify"
	"github.com/heinrichb/steamgifts-bot/internal/scorer"
	"github.com/heinrichb/steamgifts-bot/internal/state"
	sg "github.com/heinrichb/steamgifts-bot/internal/steamgifts"
)

// pointsCap is the maximum points a steamgifts account can hold.
// Points earned above this are wasted, so the bot should always
// spend enough to stay below the cap.
const pointsCap = 400

// Runner runs the bot loop for one account.
type Runner struct {
	Name          string
	Settings      config.AccountSettings
	ScorerWeights scorer.Weights
	Client        *client.Client
	Logger        *slog.Logger
	State         *state.Store
	Notifier      *notify.Notifier
	DryRun        bool

	mu          sync.RWMutex
	status      Status
	seenWins    map[string]bool
	failedCodes map[string]bool // codes rejected by the server (Missing Base Game, etc.)
}

// Status is a snapshot of what an account's runner is doing right now.
// Reads are safe under runner.Snapshot(); writes go through helper methods.
type Status struct {
	Name           string
	Username       string
	Points         int
	LastRun        time.Time
	NextRun        time.Time
	LastError      string
	EntriesAttempt int
	EntriesOK      int
	RecentEntries  []EnteredGiveaway
}

// EnteredGiveaway is one row in the recent-entries rolling window.
type EnteredGiveaway struct {
	When time.Time
	Name string
	Code string
	Cost int
}

const recentEntriesCap = 20

// Snapshot returns a copy of the current status, safe for concurrent readers.
func (r *Runner) Snapshot() Status {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := r.status
	out.Name = r.Name
	out.RecentEntries = append([]EnteredGiveaway(nil), r.status.RecentEntries...)
	return out
}

// Run loops until ctx is cancelled. If once is true, it executes a single
// pass and returns.
func (r *Runner) Run(ctx context.Context, once bool) error {
	for {
		if err := r.runOnce(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			r.recordError(err)
			r.Logger.Error("scan cycle failed", "err", err)
		}
		if once {
			return nil
		}
		pause := r.Settings.PauseDuration()
		nextRun := time.Now().Add(pause)
		r.mu.Lock()
		r.status.NextRun = nextRun
		r.mu.Unlock()
		r.Logger.Info("sleeping", "until", nextRun.Format(time.RFC3339))
		timer := time.NewTimer(pause)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func (r *Runner) recordError(err error) {
	r.mu.Lock()
	r.status.LastError = err.Error()
	r.mu.Unlock()
}

func (r *Runner) recordRun(page sg.AccountState) {
	r.mu.Lock()
	r.status.LastRun = time.Now()
	r.status.LastError = ""
	r.status.Username = page.Username
	r.status.Points = page.Points
	r.mu.Unlock()
	metrics.Points.WithLabelValues(r.Name).Set(float64(page.Points))
}

// shouldSyncSteam reports whether the per-account Steam sync interval has
// elapsed since the last successful sync. The persistent store backs this
// so restarting the bot doesn't burn the daily cooldown.
func (r *Runner) shouldSyncSteam() bool {
	if !r.Settings.SteamSyncEnabledValue() {
		return false
	}
	var last time.Time
	if r.State != nil {
		last = r.State.LastSync(r.Name)
	}
	if last.IsZero() {
		return true
	}
	interval := r.Settings.SteamSyncInterval()
	elapsed := time.Since(last)
	if elapsed < interval {
		remaining := (interval - elapsed).Round(time.Second)
		r.Logger.Debug("steam sync: cooldown",
			"last_sync", last.Format(time.RFC3339),
			"next_in", remaining.String(),
		)
		return false
	}
	return true
}

// syncSteam triggers a Steamgifts→Steam sync. Refunds points for newly-
// acquired games and filters owned games out of future listings. The site
// has its own daily cooldown, but we also enforce a configurable floor.
func (r *Runner) syncSteam(ctx context.Context, xsrf string) error {
	res, err := sg.SyncAccount(ctx, r.Client, xsrf)
	if err != nil {
		return err
	}
	now := time.Now()
	if r.State != nil {
		if perr := r.State.SetLastSync(r.Name, now); perr != nil {
			r.Logger.Warn("failed to persist last_sync", "err", perr)
		}
	}
	r.Logger.Info("synced account with steam",
		"msg", res.Msg,
		"next_in", r.Settings.SteamSyncInterval(),
	)
	metrics.SyncSucceeded.WithLabelValues(r.Name).Inc()
	return nil
}

func (r *Runner) recordEntry(g sg.Giveaway, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.EntriesAttempt++
	metrics.EntriesAttempted.WithLabelValues(r.Name).Inc()
	if ok {
		r.status.EntriesOK++
		metrics.EntriesSucceeded.WithLabelValues(r.Name).Inc()
		entry := EnteredGiveaway{When: time.Now(), Name: g.Name, Code: g.Code, Cost: g.Cost}
		r.status.RecentEntries = append(r.status.RecentEntries, entry)
		if len(r.status.RecentEntries) > recentEntriesCap {
			r.status.RecentEntries = r.status.RecentEntries[len(r.status.RecentEntries)-recentEntriesCap:]
		}
	} else {
		metrics.EntriesFailed.WithLabelValues(r.Name).Inc()
	}
}

// recordFailedCode remembers a giveaway code that the server rejected
// so we skip it in future cycles without wasting an HTTP request.
func (r *Runner) recordFailedCode(code string) {
	r.mu.Lock()
	if r.failedCodes == nil {
		r.failedCodes = make(map[string]bool)
	}
	r.failedCodes[code] = true
	r.mu.Unlock()
}

func (r *Runner) isFailedCode(code string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.failedCodes[code]
}

// fetchPage fetches and parses a single listing page.
func (r *Runner) fetchPage(ctx context.Context, url string) (sg.AccountState, []sg.Giveaway, error) {
	body, err := r.Client.Get(ctx, url)
	if err != nil {
		return sg.AccountState{}, nil, err
	}
	return sg.ParseListPage(body)
}

func (r *Runner) runOnce(ctx context.Context) error {
	minPts := r.Settings.MinPointsValue()
	maxEntries := r.Settings.MaxEntriesValue()
	allowPinned := r.Settings.EnterPinnedValue()
	filters := r.Settings.Filters
	if len(filters) == 0 {
		return errors.New("runner: no filters configured")
	}

	// --- Phase 1: scan all filter pages and collect joinable candidates ---
	syncedThisCycle := false
	seen := make(map[string]bool, 256)
	wishlistCodes := make(map[string]bool)
	candidates := make([]sg.Giveaway, 0, 256)
	var xsrf string
	var accountLevel int
	var latestPoints int

	maxPages := r.Settings.MaxPagesValue()
	for _, filter := range filters {
		basePath, err := sg.FilterURL(filter)
		if err != nil {
			return err
		}
		for pageNum := 1; pageNum <= maxPages; pageNum++ {
			pageURL := sg.WithPage(basePath, pageNum)
			page, giveaways, err := r.fetchPage(ctx, pageURL)
			if err != nil {
				return fmt.Errorf("fetch %s p%d: %w", filter, pageNum, err)
			}

			if !syncedThisCycle && r.shouldSyncSteam() {
				syncedThisCycle = true
				if err := r.syncSteam(ctx, page.XSRFToken); err != nil {
					r.Logger.Warn("steam sync failed (continuing scan)", "err", err)
				} else {
					page, giveaways, err = r.fetchPage(ctx, pageURL)
					if err != nil {
						return fmt.Errorf("post-sync refetch %s p%d: %w", filter, pageNum, err)
					}
				}
			}

			r.recordRun(page)
			xsrf = page.XSRFToken
			latestPoints = page.Points
			if page.Level > 0 {
				accountLevel = page.Level
			}

			r.Logger.Info("scanned filter",
				"filter", filter,
				"page", pageNum,
				"points", page.Points,
				"level", accountLevel,
				"giveaways", len(giveaways),
			)

			for _, g := range giveaways {
				if filter == sg.FilterWishlist {
					wishlistCodes[g.Code] = true
				}
				if seen[g.Code] || r.isFailedCode(g.Code) {
					continue
				}
				if !g.Joinable(latestPoints, minPts, accountLevel, allowPinned) {
					continue
				}
				seen[g.Code] = true
				candidates = append(candidates, g)
			}

			if len(giveaways) == 0 {
				break
			}
		}
	}

	if len(candidates) == 0 {
		r.Logger.Info("no joinable candidates found")
		return nil
	}

	// --- Phase 2: score and sort candidates by priority ---
	ranked := scorer.Rank(candidates, scorer.Context{
		WishlistCodes: wishlistCodes,
		AccountLevel:  accountLevel,
		Weights:       r.ScorerWeights,
	})
	r.Logger.Info("ranked candidates",
		"total", len(ranked),
		"top", ranked[0].Name,
		"top_score", fmt.Sprintf("%.2f", ranked[0].Score),
	)

	// --- Phase 3: enter in score order ---
	// If at the point cap, relax min_points so we don't waste regeneration.
	effectiveMin := minPts
	if latestPoints >= pointsCap {
		effectiveMin = 0
		r.Logger.Debug("at point cap, relaxing min_points to spend down")
	}

	points := latestPoints
	entered := 0
	maxPerApp := r.Settings.MaxEntriesPerAppValue()
	appEntries := make(map[string]int)
	for _, c := range ranked {
		if err := ctx.Err(); err != nil {
			r.Logger.Debug("stopping entry loop", "reason", err)
			break
		}
		if maxEntries > 0 && entered >= maxEntries {
			r.Logger.Debug("max entries per run reached", "max", maxEntries)
			break
		}
		if maxPerApp > 0 && appEntries[c.Name] >= maxPerApp {
			continue
		}
		if points-c.Cost < effectiveMin {
			continue
		}
		if r.DryRun {
			r.Logger.Info("dry-run: would enter",
				"name", c.Name, "code", c.Code, "cost", c.Cost,
				"score", fmt.Sprintf("%.2f", c.Score), "points_after", points-c.Cost)
			r.recordEntry(c.Giveaway, true)
			points -= c.Cost
			entered++
			appEntries[c.Name]++
			continue
		}
		res, err := sg.Enter(ctx, r.Client, c.Code, xsrf)
		if err != nil {
			r.recordEntry(c.Giveaway, false)
			// Remember permanent rejections so we don't retry next cycle.
			if isPermanentRejection(err) {
				r.recordFailedCode(c.Code)
				r.Logger.Debug("skipping in future", "code", c.Code, "err", err)
			} else {
				r.Logger.Warn("entry failed", "name", c.Name, "code", c.Code, "err", err)
			}
			continue
		}
		r.recordEntry(c.Giveaway, true)
		entered++
		appEntries[c.Name]++
		if res.PointsValue() > 0 {
			points = res.PointsValue()
		} else {
			points -= c.Cost
		}
		r.Logger.Info("entered",
			"name", c.Name, "code", c.Code, "cost", c.Cost,
			"score", fmt.Sprintf("%.2f", c.Score), "points_left", points)
		if points <= effectiveMin {
			r.Logger.Info("points at min after entry, stopping", "points", points)
			break
		}
	}
	r.Logger.Info("cycle complete", "entered", entered, "points_left", points)
	metrics.CyclesCompleted.WithLabelValues(r.Name).Inc()
	metrics.CandidatesScanned.WithLabelValues(r.Name).Set(float64(len(candidates)))

	if r.Notifier != nil && r.Notifier.Enabled() && !r.DryRun {
		r.checkWins(ctx)
	}
	return nil
}

// isPermanentRejection returns true for server errors that won't change
// on retry — the user is fundamentally ineligible for this giveaway.
func isPermanentRejection(err error) bool {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "Missing Base Game"):
		return true
	case strings.Contains(msg, "Exists in Account"):
		return true
	case strings.Contains(msg, "Previously Won"):
		return true
	case strings.Contains(msg, "Level") && strings.Contains(msg, "Required"):
		return true
	}
	return false
}

func (r *Runner) checkWins(ctx context.Context) {
	body, err := r.Client.Get(ctx, "/giveaways/won")
	if err != nil {
		r.Logger.Warn("failed to check wins page", "err", err)
		return
	}
	wins, err := sg.ParseWinsPage(body)
	if err != nil {
		r.Logger.Warn("failed to parse wins page", "err", err)
		return
	}

	r.mu.Lock()
	if r.seenWins == nil {
		r.seenWins = make(map[string]bool, len(wins))
		for _, w := range wins {
			r.seenWins[w.Code] = true
		}
		r.mu.Unlock()
		r.Logger.Info("win tracker initialized", "existing_wins", len(wins))
		return
	}
	r.mu.Unlock()

	for _, w := range wins {
		r.mu.Lock()
		already := r.seenWins[w.Code]
		if !already {
			r.seenWins[w.Code] = true
		}
		r.mu.Unlock()
		if already {
			continue
		}
		metrics.WinsDetected.WithLabelValues(r.Name).Inc()
		r.Logger.Info("won a giveaway!", "game", w.Name, "url", w.URL)
		if err := r.Notifier.SendWin(ctx, notify.Win{
			GameName:    w.Name,
			GiveawayURL: r.Client.BaseURL() + w.URL,
			AccountName: r.Name,
		}); err != nil {
			r.Logger.Warn("failed to send win notification", "err", err)
		}
	}
}
