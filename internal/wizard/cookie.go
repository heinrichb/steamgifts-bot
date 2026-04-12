package wizard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/pkg/browser"

	"github.com/heinrichb/steamgifts-bot/internal/client"
	"github.com/heinrichb/steamgifts-bot/internal/config"
	sg "github.com/heinrichb/steamgifts-bot/internal/steamgifts"
)

// AccountInput is the prompt context for capturing one account.
type AccountInput struct {
	DefaultName string
	UserAgent   string
}

// CaptureAccount runs the per-account piece of the wizard:
//
//   - opens steamgifts.com in the user's default browser
//   - prints clear, plain-language DevTools instructions
//   - prompts for the cookie
//   - validates it live by signing in and reading the username + points
//   - prompts for an optional friendly account name
//
// It returns a fully-populated config.Account ready to append to the config.
func CaptureAccount(ctx context.Context, in AccountInput) (config.Account, error) {
	const loginURL = "https://www.steamgifts.com/?login"

	// Step 1: kick the browser open and explain what to do. We don't fail
	// if browser.OpenURL errors — the user can paste the URL by hand.
	openErr := browser.OpenURL(loginURL)
	intro := strings.Join([]string{
		"Sign in to steamgifts.com in the browser window that just opened, then come back here.",
		"",
		"To copy your cookie:",
		"  1. Press F12 (or right-click → Inspect)",
		"  2. Open the 'Application' tab in Chrome — or 'Storage' in Firefox",
		"  3. Expand 'Cookies' → 'https://www.steamgifts.com'",
		"  4. Click the 'PHPSESSID' row",
		"  5. Copy the long Value string",
	}, "\n")
	if openErr != nil {
		intro = "Open this URL in your browser to sign in:\n  " + loginURL + "\n\n" + intro
	}

	if err := huh.NewNote().
		Title("Step 1: capture your cookie").
		Description(intro).
		Next(true).
		WithTheme(huh.ThemeCharm()).
		Run(); err != nil {
		return config.Account{}, err
	}

	// Step 2: paste + validate. Loop until the cookie passes or the user quits.
	var cookie string
	var validated *sg.AccountState
	for {
		err := huh.NewInput().
			Title("Paste your PHPSESSID cookie").
			Description("Just the value — no quotes, no 'PHPSESSID=' prefix.").
			EchoMode(huh.EchoModePassword).
			Value(&cookie).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("cookie cannot be empty")
				}
				return nil
			}).
			WithTheme(huh.ThemeCharm()).
			Run()
		if err != nil {
			return config.Account{}, err
		}

		fmt.Println("→ checking cookie against steamgifts.com…")
		state, err := validateCookie(ctx, cookie, in.UserAgent)
		if err == nil {
			fmt.Printf("✓ signed in as %s — %d points\n\n", state.Username, state.Points)
			validated = state
			break
		}

		fmt.Printf("✗ cookie didn't work: %s\n", err)
		retry := true
		_ = huh.NewConfirm().
			Title("Try again?").
			Affirmative("Try a different cookie").
			Negative("Quit setup").
			Value(&retry).
			WithTheme(huh.ThemeCharm()).
			Run()
		if !retry {
			return config.Account{}, errors.New("cookie capture cancelled")
		}
	}

	// Step 3: pick a friendly name (default to the detected steamgifts username).
	name := validated.Username
	if name == "" {
		name = in.DefaultName
	}
	if err := huh.NewInput().
		Title("Account label").
		Description("A short name to identify this account in logs and the dashboard.").
		Value(&name).
		Validate(func(s string) error {
			if strings.TrimSpace(s) == "" {
				return errors.New("name cannot be empty")
			}
			return nil
		}).
		WithTheme(huh.ThemeCharm()).
		Run(); err != nil {
		return config.Account{}, err
	}

	return config.Account{Name: strings.TrimSpace(name), Cookie: strings.TrimSpace(cookie)}, nil
}

func validateCookie(ctx context.Context, cookie, userAgent string) (*sg.AccountState, error) {
	if userAgent == "" {
		userAgent = config.DefaultUserAgent
	}
	c, err := client.New(cookie, userAgent)
	if err != nil {
		return nil, err
	}
	timeout, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	body, err := c.Get(timeout, "/")
	if err != nil {
		return nil, err
	}
	state, _, err := sg.ParseListPage(body)
	if err != nil {
		return nil, err
	}
	if state.Username == "" {
		return nil, errors.New("signed-in username not found — cookie likely expired or wrong value")
	}
	return &state, nil
}
