package account

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/heinrichb/steamgifts-bot/internal/client"
	"github.com/heinrichb/steamgifts-bot/internal/config"
	logpkg "github.com/heinrichb/steamgifts-bot/internal/log"
)

// Orchestrator owns one Runner per configured account and starts/stops them
// as a group.
type Orchestrator struct {
	Runners []*Runner
}

// Build constructs an Orchestrator from a Config. Each account gets its own
// HTTP client, cookie jar, and rate limiter.
func Build(cfg *config.Config, logger *slog.Logger, dryRun bool) (*Orchestrator, error) {
	orch := &Orchestrator{}
	for i := range cfg.Accounts {
		acct := cfg.Accounts[i]
		settings := cfg.Resolved(i)
		log := logpkg.Account(logger, acct.Name)
		c, err := client.New(acct.Cookie, settings.UserAgent,
			client.WithLogger(log),
		)
		if err != nil {
			return nil, fmt.Errorf("account %q: %w", acct.Name, err)
		}
		orch.Runners = append(orch.Runners, &Runner{
			Name:     acct.Name,
			Settings: settings,
			Client:   c,
			Logger:   log,
			DryRun:   dryRun,
		})
	}
	return orch, nil
}

// Run starts each runner in its own goroutine. Returns when ctx is cancelled
// or every once-mode runner has finished.
func (o *Orchestrator) Run(ctx context.Context, once bool) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(o.Runners))
	for _, r := range o.Runners {
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
	out := make([]Status, len(o.Runners))
	for i, r := range o.Runners {
		out[i] = r.Snapshot()
	}
	return out
}
