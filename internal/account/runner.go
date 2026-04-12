// Package account drives a single steamgifts.com account: it polls the
// configured filter pages on a schedule and enters joinable giveaways
// until points are exhausted or the per-run cap is reached.
package account

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/heinrichb/steamgifts-bot/internal/client"
	"github.com/heinrichb/steamgifts-bot/internal/config"
	"github.com/heinrichb/steamgifts-bot/internal/state"
	sg "github.com/heinrichb/steamgifts-bot/internal/steamgifts"
)

// Runner runs the bot loop for one account.
type Runner struct {
	Name     string
	Settings config.AccountSettings
	Client   *client.Client
	Logger   *slog.Logger
	State    *state.Store // persistent per-account state (last sync time, etc.)

	// DryRun, when true, parses giveaways and logs candidates but never
	// submits an entry. The wizard, `check`, and `--dry-run` all use this.
	DryRun bool

	mu     sync.RWMutex
	status Status
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
		r.scheduleNext(pause)
		r.Logger.Info("sleeping", "until", time.Now().Add(pause).Format(time.RFC3339))
		timer := time.NewTimer(pause)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

func (r *Runner) scheduleNext(pause time.Duration) {
	r.mu.Lock()
	r.status.NextRun = time.Now().Add(pause)
	r.mu.Unlock()
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
		r.Logger.Debug("steam sync: cooldown",
			"last_sync", last.Format(time.RFC3339),
			"next_in", (interval - elapsed).Round(time.Second),
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
	return nil
}

func (r *Runner) recordEntry(g sg.Giveaway, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.EntriesAttempt++
	if ok {
		r.status.EntriesOK++
		entry := EnteredGiveaway{When: time.Now(), Name: g.Name, Code: g.Code, Cost: g.Cost}
		r.status.RecentEntries = append(r.status.RecentEntries, entry)
		if len(r.status.RecentEntries) > recentEntriesCap {
			r.status.RecentEntries = r.status.RecentEntries[len(r.status.RecentEntries)-recentEntriesCap:]
		}
	}
}

func (r *Runner) runOnce(ctx context.Context) error {
	minPts := r.Settings.MinPointsValue()
	maxEntries := r.Settings.MaxEntriesValue()
	allowPinned := r.Settings.EnterPinnedValue()
	filters := r.Settings.Filters
	if len(filters) == 0 {
		return errors.New("runner: no filters configured")
	}

	enteredThisRun := 0
	syncedThisCycle := false
	// Per-cycle dedupe by giveaway code: the same listing can show up under
	// multiple filters and we don't want to double-POST.
	enteredThisCycle := make(map[string]bool)
	// Carry simulated spend across filters in dry-run mode so points_after
	// reflects cumulative cost. Stays zero in real mode (server is the truth).
	dryRunSpend := 0

	for _, filter := range filters {
		if maxEntries > 0 && enteredThisRun >= maxEntries {
			r.Logger.Debug("max entries per run reached, stopping cycle", "max", maxEntries)
			return nil
		}

		path, err := sg.FilterURL(filter)
		if err != nil {
			return err
		}
		body, err := r.Client.Get(ctx, path)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", filter, err)
		}
		page, giveaways, err := sg.ParseListPage(body)
		if err != nil {
			return fmt.Errorf("parse %s: %w", filter, err)
		}

		// Sync once per cycle, piggybacking on the xsrf token from the
		// first filter, then re-read the same filter so the loop sees
		// post-sync points and giveaway list.
		if !syncedThisCycle && r.shouldSyncSteam() {
			syncedThisCycle = true
			if err := r.syncSteam(ctx, page.XSRFToken); err != nil {
				r.Logger.Warn("steam sync failed (continuing scan)", "err", err)
			} else {
				body, err = r.Client.Get(ctx, path)
				if err != nil {
					return fmt.Errorf("post-sync refetch %s: %w", filter, err)
				}
				page, giveaways, err = sg.ParseListPage(body)
				if err != nil {
					return fmt.Errorf("post-sync parse %s: %w", filter, err)
				}
			}
		}

		r.recordRun(page)
		r.Logger.Info("scanned filter",
			"filter", filter,
			"points", page.Points,
			"level", page.Level,
			"username", page.Username,
			"giveaways", len(giveaways),
		)

		points := page.Points - dryRunSpend
		if points <= minPts {
			r.Logger.Info("points at or below min, skipping further filters", "points", points, "min", minPts)
			return nil
		}

		for _, g := range giveaways {
			if maxEntries > 0 && enteredThisRun >= maxEntries {
				return nil
			}
			if enteredThisCycle[g.Code] {
				r.Logger.Debug("skipping duplicate across filters",
					"name", g.Name, "code", g.Code, "filter", filter)
				continue
			}
			if !g.Joinable(points, minPts, page.Level, allowPinned) {
				continue
			}
			if r.DryRun {
				r.Logger.Info("dry-run: would enter",
					"name", g.Name, "code", g.Code, "cost", g.Cost, "points_after", points-g.Cost)
				enteredThisCycle[g.Code] = true
				r.recordEntry(g, true)
				points -= g.Cost
				dryRunSpend += g.Cost
				enteredThisRun++
				continue
			}
			res, err := sg.Enter(ctx, r.Client, g.Code, page.XSRFToken)
			if err != nil {
				r.recordEntry(g, false)
				r.Logger.Warn("entry failed", "name", g.Name, "code", g.Code, "err", err)
				continue
			}
			enteredThisCycle[g.Code] = true
			r.recordEntry(g, true)
			enteredThisRun++
			if res.PointsValue() > 0 {
				points = res.PointsValue()
			} else {
				points -= g.Cost
			}
			r.Logger.Info("entered",
				"name", g.Name, "code", g.Code, "cost", g.Cost, "points_left", points)
			if points <= minPts {
				r.Logger.Info("points at min after entry, stopping cycle", "points", points)
				return nil
			}
		}
	}
	return nil
}
