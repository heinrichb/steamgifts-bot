// Package wizard implements the friendly first-run setup flow.
//
// The flow is:
//
//  1. Welcome screen
//  2. Per-account cookie capture (with browser-assisted instructions
//     and live validation against steamgifts.com)
//  3. Per-account settings
//  4. Optional add-another-account loop
//  5. "How should it run?" — foreground / install service / save only
//  6. Confirm + write config
package wizard

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/heinrichb/steamgifts-bot/internal/config"
	"github.com/heinrichb/steamgifts-bot/internal/service"
)

// Options drives a Run().
type Options struct {
	Config   *config.Config // existing config (may have zero accounts)
	SavePath string         // where to write the result
}

// Result is what Run returns.
type Result struct {
	Saved bool
	Path  string
}

// Run executes the full first-run wizard.
func Run(ctx context.Context, opts Options) (Result, error) {
	if opts.Config == nil {
		d := config.Defaults()
		opts.Config = &d
	}

	if err := welcome(ctx); err != nil {
		return Result{}, err
	}

	editing := len(opts.Config.Accounts) > 0
	if editing {
		keepExisting, err := runConfirm(
			fmt.Sprintf("Found %d existing account(s) in your config.", len(opts.Config.Accounts)),
			"Keep them and just add more, or start fresh?",
			"Keep existing",
			"Start fresh",
		)
		if err != nil {
			return Result{}, err
		}
		if !keepExisting {
			opts.Config.Accounts = nil
		}
	}

	for {
		acct, err := CaptureAccount(ctx, AccountInput{
			DefaultName: fmt.Sprintf("account-%d", len(opts.Config.Accounts)+1),
			UserAgent:   opts.Config.Defaults.UserAgent,
		})
		if err != nil {
			return Result{}, err
		}
		opts.Config.Accounts = append(opts.Config.Accounts, acct)

		addAnother, err := runConfirm(
			"Add another account?",
			"",
			"Yes, add another",
			"No, I'm done",
		)
		if err != nil {
			return Result{}, err
		}
		if !addAnother {
			break
		}
	}

	if err := globalSettings(opts.Config); err != nil {
		return Result{}, err
	}

	if err := opts.Config.Validate(); err != nil {
		return Result{}, fmt.Errorf("the wizard produced an invalid config: %w", err)
	}

	save, err := runConfirm(
		fmt.Sprintf("Save config to: %s", opts.SavePath),
		"Existing files at this path will be overwritten.",
		"Save",
		"Cancel",
	)
	if err != nil {
		return Result{}, err
	}
	if !save {
		return Result{Saved: false, Path: opts.SavePath}, nil
	}
	if err := saveConfigYAML(opts.Config, opts.SavePath); err != nil {
		return Result{}, err
	}

	if err := offerService(ctx); err != nil {
		fmt.Println("(skipped service install:", err, ")")
	}

	return Result{Saved: true, Path: opts.SavePath}, nil
}

func welcome(_ context.Context) error {
	ready, err := runConfirm(
		"Welcome to SteamGifts Bot",
		strings.Join([]string{
			"This wizard will set up the bot in a few quick steps:",
			"",
			"  1. Sign in to steamgifts.com in your browser",
			"  2. Copy your PHPSESSID cookie",
			"  3. Choose how the bot should behave",
			"",
			"You can always re-run this wizard with `steamgifts-bot setup`.",
		}, "\n"),
		"Let's go",
		"Quit",
	)
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("setup cancelled")
	}
	return nil
}

func globalSettings(cfg *config.Config) error {
	if err := runNote("Global defaults",
		"These apply to every account unless you override them later.\nYou can edit config.yml by hand any time."); err != nil {
		return err
	}

	min, err := runInput(
		"Minimum points to keep",
		"The bot will stop entering when spending more would drop you below this. (0-400)",
		strconv.Itoa(deref(cfg.Defaults.MinPoints, 50)),
		false,
		intRange(0, 400),
	)
	if err != nil {
		return err
	}

	pause, err := runInput(
		"Pause between scans (minutes)",
		"How long to wait between scan cycles. 15 is a friendly default.",
		strconv.Itoa(deref(cfg.Defaults.PauseMinutes, 15)),
		false,
		intRange(1, 1440),
	)
	if err != nil {
		return err
	}

	maxEntries, err := runInput(
		"Max entries per scan",
		"Safety cap. 25 is plenty for most accounts.",
		strconv.Itoa(deref(cfg.Defaults.MaxEntriesPerRun, 25)),
		false,
		intRange(0, 1000),
	)
	if err != nil {
		return err
	}

	pinned, err := runConfirm(
		"Include pinned giveaways?",
		"Pinned/featured giveaways are usually high-entry. Most users leave this off.",
		"Include them",
		"Skip them",
	)
	if err != nil {
		return err
	}

	cfg.Defaults.MinPoints = intPtr(atoi(min))
	cfg.Defaults.PauseMinutes = intPtr(atoi(pause))
	cfg.Defaults.MaxEntriesPerRun = intPtr(atoi(maxEntries))
	cfg.Defaults.EnterPinned = boolPtr(pinned)
	return nil
}

func offerService(_ context.Context) error {
	if !service.Supported() {
		return nil
	}
	install, err := runConfirm(
		"Install as a background service?",
		serviceDescription(),
		"Yes, install",
		"No thanks",
	)
	if err != nil {
		return err
	}
	if !install {
		return nil
	}
	path, err := service.Install()
	if err != nil {
		return err
	}
	fmt.Println("Installed:", path)
	fmt.Println("  Uninstall any time with: steamgifts-bot service uninstall")
	return nil
}

func serviceDescription() string {
	switch service.Platform() {
	case "windows":
		return "Adds a small script to your Startup folder so the bot starts\nminimized when you log in. No admin required, fully reversible."
	case "linux":
		return "Writes a systemd user unit and enables it.\nRuns as you, fully reversible."
	case "darwin":
		return "Writes a LaunchAgent plist so the bot starts on login.\nRuns as you, fully reversible."
	default:
		return "Installs a small launcher so the bot starts automatically."
	}
}

func intRange(lo, hi int) func(string) error {
	return func(s string) error {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return errors.New("must be a number")
		}
		if n < lo || n > hi {
			return fmt.Errorf("must be between %d and %d", lo, hi)
		}
		return nil
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func intPtr(i int) *int    { return &i }
func boolPtr(b bool) *bool { return &b }

func deref(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}
func derefBool(p *bool) bool { return p != nil && *p }
