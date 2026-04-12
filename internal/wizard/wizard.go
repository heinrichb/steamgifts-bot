// Package wizard implements the friendly first-run setup flow.
//
// It is built on charmbracelet/huh so it works in any modern terminal —
// cmd.exe, PowerShell, Windows Terminal, gnome-terminal, etc. — without
// requiring a separate GUI binary.
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

	"github.com/charmbracelet/huh"

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
		var keepExisting bool
		err := huh.NewConfirm().
			Title(fmt.Sprintf("Found %d existing account(s) in your config.", len(opts.Config.Accounts))).
			Description("Keep them and just add more, or start fresh?").
			Affirmative("Keep existing").
			Negative("Start fresh").
			Value(&keepExisting).
			WithTheme(huh.ThemeCharm()).
			Run()
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

		var addAnother bool
		err = huh.NewConfirm().
			Title("Add another account?").
			Affirmative("Yes, add another").
			Negative("No, I'm done").
			Value(&addAnother).
			WithTheme(huh.ThemeCharm()).
			Run()
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

	var save bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("Save config to:\n  %s", opts.SavePath)).
		Description("Existing files at this path will be overwritten.").
		Affirmative("Save").
		Negative("Cancel").
		Value(&save).
		WithTheme(huh.ThemeCharm()).
		Run()
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
		// Service install is optional UX sugar — surface but don't fail the wizard.
		fmt.Println("(skipped service install:", err, ")")
	}

	return Result{Saved: true, Path: opts.SavePath}, nil
}

func welcome(_ context.Context) error {
	var ready bool
	return huh.NewConfirm().
		Title("Welcome to steamgifts-bot 👋").
		Description(strings.Join([]string{
			"This wizard will set up the bot in a few quick steps:",
			"",
			"  1. Sign in to steamgifts.com in your browser",
			"  2. Copy your PHPSESSID cookie",
			"  3. Choose how the bot should behave",
			"",
			"You can always re-run this wizard with `steamgifts-bot setup`.",
		}, "\n")).
		Affirmative("Let's go").
		Negative("Quit").
		Value(&ready).
		WithTheme(huh.ThemeCharm()).
		Run()
}

func globalSettings(cfg *config.Config) error {
	min := strconv.Itoa(deref(cfg.Defaults.MinPoints, 50))
	pause := strconv.Itoa(deref(cfg.Defaults.PauseMinutes, 15))
	maxEntries := strconv.Itoa(deref(cfg.Defaults.MaxEntriesPerRun, 25))
	pinned := derefBool(cfg.Defaults.EnterPinned)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Global defaults").
				Description("These apply to every account unless you override them later. You can edit `config.yaml` by hand any time."),

			huh.NewInput().
				Title("Minimum points to keep").
				Description("The bot will stop entering when spending more would drop you below this. (0–400)").
				Value(&min).
				Validate(intRange(0, 400)),

			huh.NewInput().
				Title("Pause between scans (minutes)").
				Description("How long to wait between scan cycles. 15 is a friendly default.").
				Value(&pause).
				Validate(intRange(1, 1440)),

			huh.NewInput().
				Title("Max entries per scan").
				Description("Safety cap. 25 is plenty for most accounts.").
				Value(&maxEntries).
				Validate(intRange(0, 1000)),

			huh.NewConfirm().
				Title("Include pinned giveaways?").
				Description("Pinned/featured giveaways are usually high-entry. Most users leave this off.").
				Affirmative("Include them").
				Negative("Skip them").
				Value(&pinned),
		),
	).WithTheme(huh.ThemeCharm())
	if err := form.Run(); err != nil {
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
	var install bool
	if err := huh.NewConfirm().
		Title("Install as a background service?").
		Description(serviceDescription()).
		Affirmative("Yes, install").
		Negative("No thanks").
		Value(&install).
		WithTheme(huh.ThemeCharm()).
		Run(); err != nil {
		return err
	}
	if !install {
		return nil
	}
	path, err := service.Install()
	if err != nil {
		return err
	}
	fmt.Println("✓ Installed:", path)
	fmt.Println("  Uninstall any time with: steamgifts-bot service uninstall")
	return nil
}

func serviceDescription() string {
	switch service.Platform() {
	case "windows":
		return "I'll create a per-user Scheduled Task that starts the bot when you log in. No admin required, fully reversible."
	case "linux":
		return "I'll write a systemd user unit and enable it. Runs as you, fully reversible."
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
