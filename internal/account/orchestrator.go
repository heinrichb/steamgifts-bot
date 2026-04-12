package account

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/heinrichb/steamgifts-bot/internal/client"
	"github.com/heinrichb/steamgifts-bot/internal/config"
	logpkg "github.com/heinrichb/steamgifts-bot/internal/log"
	"github.com/heinrichb/steamgifts-bot/internal/notify"
	"github.com/heinrichb/steamgifts-bot/internal/scorer"
	"github.com/heinrichb/steamgifts-bot/internal/state"
)

// Orchestrator owns one Runner per configured account and starts/stops them
// as a group.
type Orchestrator struct {
	runners []*Runner
}

// Build constructs an Orchestrator from a Config. Each account gets its own
// HTTP client, cookie jar, and rate limiter. The state store, when non-nil,
// is shared across runners and provides persistence for last-sync timestamps.
func Build(cfg *config.Config, logger *slog.Logger, store *state.Store, notif *notify.Notifier, dryRun bool) (*Orchestrator, error) {
	orch := &Orchestrator{}
	for i := range cfg.Accounts {
		acct := cfg.Accounts[i]
		settings := cfg.Resolved(i)
		log := logpkg.Account(logger, acct.Name)
		clientOpts := []client.Option{client.WithLogger(log)}
		if settings.ProxyURL != "" {
			clientOpts = append(clientOpts, client.WithProxy(settings.ProxyURL))
			log.Info("using proxy", "proxy", settings.ProxyURL)
		}
		c, err := client.New(acct.Cookie, settings.UserAgent, clientOpts...)
		if err != nil {
			return nil, fmt.Errorf("account %q: %w", acct.Name, err)
		}
		// Per-account scorer: start from global, override with per-account.
		sw := cfg.Scorer
		if settings.Scorer != nil {
			sw = mergedScorerWeights(sw, *settings.Scorer)
		}
		orch.runners = append(orch.runners, &Runner{
			Name:          acct.Name,
			Settings:      settings,
			ScorerWeights: scorerWeightsFromConfig(sw),
			Client:        c,
			Logger:        log,
			State:         store,
			Notifier:      notif,
			DryRun:        dryRun,
		})
	}
	return orch, nil
}

func mergedScorerWeights(base, override config.ScorerWeights) config.ScorerWeights {
	if override.Wishlist != nil {
		base.Wishlist = override.Wishlist
	}
	if override.Sniper != nil {
		base.Sniper = override.Sniper
	}
	if override.SniperHours != nil {
		base.SniperHours = override.SniperHours
	}
	if override.Level != nil {
		base.Level = override.Level
	}
	if override.CostEfficiency != nil {
		base.CostEfficiency = override.CostEfficiency
	}
	return base
}

func scorerWeightsFromConfig(sw config.ScorerWeights) scorer.Weights {
	// Start with defaults, then apply explicit overrides. This lets users
	// set a weight to 0 to disable a component — without this, zero would
	// be indistinguishable from "not set."
	w := scorer.Weights{
		Wishlist:    scorer.DefaultWishlistWeight,
		Sniper:      scorer.DefaultSniperWeight,
		SniperHours: scorer.DefaultSniperHours,
		Level:       scorer.DefaultLevelWeight,
		Cost:        scorer.DefaultCostWeight,
	}
	if sw.Wishlist != nil {
		w.Wishlist = *sw.Wishlist
	}
	if sw.Sniper != nil {
		w.Sniper = *sw.Sniper
	}
	if sw.SniperHours != nil {
		w.SniperHours = *sw.SniperHours
	}
	if sw.Level != nil {
		w.Level = *sw.Level
	}
	if sw.CostEfficiency != nil {
		w.Cost = *sw.CostEfficiency
	}
	return w
}

// Run starts each runner in its own goroutine. Returns when ctx is cancelled
// or every once-mode runner has finished.
func (o *Orchestrator) Run(ctx context.Context, once bool) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(o.runners))
	for _, r := range o.runners {
		r := r
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.Run(ctx, once); err != nil {
				errs <- fmt.Errorf("%s: %w", r.Name, err)
			}
		}()
	}
	wg.Wait()
	close(errs)
	var first error
	for e := range errs {
		if first == nil {
			first = e
		}
	}
	return first
}

// Snapshot returns the live status of every runner — used by the TUI.
func (o *Orchestrator) Snapshot() []Status {
	out := make([]Status, len(o.runners))
	for i, r := range o.runners {
		out[i] = r.Snapshot()
	}
	return out
}
