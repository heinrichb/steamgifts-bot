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
	sg "github.com/heinrichb/steamgifts-bot/internal/steamgifts"
)

// Runner runs the bot loop for one account.
type Runner struct {
	Name     string
	Settings config.AccountSettings
	Client   *client.Client
	Logger   *slog.Logger

	// DryRun, when true, parses giveaways and logs candidates but never
	// submits an entry. The wizard, `check`, and `--dry-run` all use this.
	DryRun bool

	mu       sync.RWMutex
	status   Status
	lastSync time.Time // last successful Steam sync attempt
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
	out.RecentEntries = append([]EnteredGiveaway(nil), r.status.RecentEntries...)
	return out
}

// Run loops until ctx is cancelled. If once is true, it executes a single
// pass and returns.
func (r *Runner) Run(ctx context.Context, once bool) error {
	r.setName()
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
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(pause):
		}
	}
}

func (r *Runner) setName() {
	r.mu.Lock()
	r.status.Name = r.Name
	r.mu.Unlock()
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

func (r *Runner) recordRun(state sg.AccountState) {
	r.mu.Lock()
	r.status.LastRun = time.Now()
	r.status.LastError = ""
	r.status.Username = state.Username
	r.status.Points = state.Points
	r.mu.Unlock()
}

// shouldSyncSteam reports whether the per-account Steam sync interval has
// elapsed since the last successful sync (or never synced).
func (r *Runner) shouldSyncSteam() bool {
	if !r.Settings.SteamSyncEnabledValue() {
		return false
	}
	r.mu.RLock()
	last := r.lastSync
	r.mu.RUnlock()
	if last.IsZero() {
		return true
	}
	return time.Since(last) >= r.Settings.SteamSyncInterval()
}

// syncSteam triggers a Steamgifts→Steam sync. Refunds points for newly-
// acquired games and filters owned games out of future listings. The site
// has its own daily cooldown, but we also enforce a configurable floor.
func (r *Runner) syncSteam(ctx context.Context, xsrf string) error {
	res, err := sg.SyncAccount(ctx, r.Client, xsrf)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.lastSync = time.Now()
	r.mu.Unlock()
	r.Logger.Info("synced account with steam", "msg", res.Msg)
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
	min := r.Settings.MinPointsValue()
	maxEntries := r.Settings.MaxEntriesValue()
	allowPinned := r.Settings.EnterPinnedValue()
	filters := r.Settings.Filters
	if len(filters) == 0 {
		return errors.New("runner: no filters configured")
	}

	enteredThisRun := 0
	syncedThisCycle := false

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
		state, giveaways, err := sg.ParseListPage(body)
		if err != nil {
			return fmt.Errorf("parse %s: %w", filter, err)
		}

		// Trigger a Steam sync at most once per cycle, on the first filter.
		// We piggyback on the page we already fetched (which gave us the
		// xsrf token), then re-fetch this filter so the rest of the cycle
		// sees post-sync points and giveaway list.
		if !syncedThisCycle && r.shouldSyncSteam() {
			syncedThisCycle = true
			if err := r.syncSteam(ctx, state.XSRFToken); err != nil {
				r.Logger.Warn("steam sync failed (continuing scan)", "err", err)
			} else {
				body, err = r.Client.Get(ctx, path)
				if err != nil {
					return fmt.Errorf("post-sync refetch %s: %w", filter, err)
				}
				state, giveaways, err = sg.ParseListPage(body)
				if err != nil {
					return fmt.Errorf("post-sync parse %s: %w", filter, err)
				}
			}
		}

		r.recordRun(state)
		r.Logger.Info("scanned filter",
			"filter", filter,
			"points", state.Points,
			"username", state.Username,
			"giveaways", len(giveaways),
		)

		if state.Points <= min {
			r.Logger.Info("points at or below min, skipping further filters", "points", state.Points, "min", min)
			return nil
		}

		points := state.Points
		for _, g := range giveaways {
			if maxEntries > 0 && enteredThisRun >= maxEntries {
				return nil
			}
			if !g.Joinable(points, min, allowPinned) {
				continue
			}
			if r.DryRun {
				r.Logger.Info("dry-run: would enter",
					"name", g.Name, "code", g.Code, "cost", g.Cost, "points_after", points-g.Cost)
				r.recordEntry(g, true)
				points -= g.Cost
				enteredThisRun++
				continue
			}
			res, err := sg.Enter(ctx, r.Client, g.Code, state.XSRFToken)
			if err != nil {
				r.recordEntry(g, false)
				r.Logger.Warn("entry failed", "name", g.Name, "code", g.Code, "err", err)
				continue
			}
			r.recordEntry(g, true)
			enteredThisRun++
			if res.Points > 0 {
				points = res.Points
				r.mu.Lock()
				r.status.Points = points
				r.mu.Unlock()
			} else {
				points -= g.Cost
			}
			r.Logger.Info("entered",
				"name", g.Name, "code", g.Code, "cost", g.Cost, "points_left", points)
			if points <= min {
				r.Logger.Info("points at min after entry, stopping cycle", "points", points)
				return nil
			}
		}
	}
	return nil
}
