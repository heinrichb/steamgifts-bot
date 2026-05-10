package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"

	"github.com/heinrichb/steamgifts-bot/internal/client"
	"github.com/heinrichb/steamgifts-bot/internal/config"
	"github.com/heinrichb/steamgifts-bot/internal/service"
	sg "github.com/heinrichb/steamgifts-bot/internal/steamgifts"
)

type wizMode int

const (
	wizModeAddAccount wizMode = iota
	wizModeSetup
)

type wizStep int

const (
	wizCookieNote wizStep = iota
	wizCookieInput
	wizCookieValidating
	wizCookieRetry
	wizAccountName
	wizAddAnother
	wizSettingsNote
	wizMinPoints
	wizPauseMinutes
	wizMaxEntries
	wizPinnedConfirm
	wizSaveConfirm
	wizServiceConfirm
	wizResult
)

type cookieValidatedMsg struct {
	state *sg.AccountState
	err   error
}

type wizardFlowModel struct {
	mode    wizMode
	step    wizStep
	cfg     *config.Config
	cfgPath string

	confirm confirmField
	input   inputField
	note    noteField

	cookie    string
	validated *sg.AccountState
	validErr  string

	resultMsg string

	done      bool
	cancelled bool
}

func newWizardFlow(mode wizMode, cfg *config.Config, cfgPath string) wizardFlowModel {
	m := wizardFlowModel{
		mode:    mode,
		cfg:     cfg,
		cfgPath: cfgPath,
	}
	if cfg == nil {
		d := config.Defaults()
		m.cfg = &d
	}
	if m.cfgPath == "" {
		m.cfgPath = defaultSavePath()
	}
	m.enterStep(wizCookieNote)
	return m
}

func (m *wizardFlowModel) enterStep(step wizStep) {
	m.step = step
	switch step {
	case wizCookieNote:
		_ = browser.OpenURL("https://www.steamgifts.com/?login")
		m.note.reset("Capture your cookie", strings.Join([]string{
			"Sign in to steamgifts.com in the browser that just opened.",
			"",
			"To copy your cookie:",
			"  1. Press F12 (or right-click > Inspect)",
			"  2. Open the 'Application' tab (Chrome) or 'Storage' (Firefox)",
			"  3. Expand 'Cookies' > 'https://www.steamgifts.com'",
			"  4. Click the 'PHPSESSID' row",
			"  5. Copy the long Value string",
		}, "\n"))

	case wizCookieInput:
		m.input.reset("Paste your PHPSESSID cookie",
			"Just the value — no quotes, no 'PHPSESSID=' prefix.",
			"", true, func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("cookie cannot be empty")
				}
				return nil
			})

	case wizCookieValidating:
		m.validErr = ""

	case wizCookieRetry:
		m.confirm.reset("Cookie didn't work: "+m.validErr,
			"", "Try a different cookie", "Cancel")

	case wizAccountName:
		name := ""
		if m.validated != nil {
			name = m.validated.Username
		}
		if name == "" {
			name = fmt.Sprintf("account-%d", len(m.cfg.Accounts)+1)
		}
		m.input.reset("Account label",
			"A short name to identify this account in logs and the dashboard.",
			name, false, func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("name cannot be empty")
				}
				return nil
			})

	case wizAddAnother:
		m.confirm.reset("Add another account?", "", "Yes, add another", "No, I'm done")

	case wizSettingsNote:
		m.note.reset("Global defaults",
			"These apply to every account unless you override them later.\nYou can edit config.yml by hand any time.")

	case wizMinPoints:
		m.input.reset("Minimum points to keep",
			"The bot will stop entering below this threshold. (0-400)",
			strconv.Itoa(derefInt(m.cfg.Defaults.MinPoints, 50)),
			false, intRangeValidator(0, 400))

	case wizPauseMinutes:
		m.input.reset("Pause between scans (minutes)",
			"How long to wait between scan cycles. 15 is a friendly default.",
			strconv.Itoa(derefInt(m.cfg.Defaults.PauseMinutes, 15)),
			false, intRangeValidator(1, 1440))

	case wizMaxEntries:
		m.input.reset("Max entries per scan",
			"Safety cap. 25 is plenty for most accounts.",
			strconv.Itoa(derefInt(m.cfg.Defaults.MaxEntriesPerRun, 25)),
			false, intRangeValidator(0, 1000))

	case wizPinnedConfirm:
		m.confirm.reset("Include pinned giveaways?",
			"Pinned/featured giveaways are usually high-entry. Most users leave this off.",
			"Include them", "Skip them")

	case wizSaveConfirm:
		m.confirm.reset(fmt.Sprintf("Save config to %s?", m.cfgPath),
			"Existing files at this path will be overwritten.",
			"Save", "Cancel")

	case wizServiceConfirm:
		if !service.Supported() {
			m.resultMsg = statusOK("Config saved to " + m.cfgPath)
			m.step = wizResult
			return
		}
		m.confirm.reset("Install as a background service?",
			wizServiceDescription(), "Yes, install", "No thanks")

	case wizResult:
	}
}

func (m wizardFlowModel) Init() tea.Cmd {
	if m.step == wizCookieInput {
		return m.input.Init()
	}
	return nil
}

func (m wizardFlowModel) Update(msg tea.Msg) (appPage, tea.Cmd) {
	switch m.step {
	case wizCookieNote:
		m.note.Update(msg)
		if m.note.cancelled {
			m.cancelled = true
			return m, nil
		}
		if m.note.done {
			m.enterStep(wizCookieInput)
			return m, m.input.Init()
		}

	case wizCookieInput:
		cmd := m.input.Update(msg)
		if m.input.cancelled {
			m.cancelled = true
			return m, nil
		}
		if m.input.done {
			m.cookie = strings.TrimSpace(m.input.ti.Value())
			m.enterStep(wizCookieValidating)
			return m, m.validateCookie()
		}
		return m, cmd

	case wizCookieValidating:
		if vmsg, ok := msg.(cookieValidatedMsg); ok {
			if vmsg.err != nil {
				m.validErr = vmsg.err.Error()
				m.enterStep(wizCookieRetry)
			} else {
				m.validated = vmsg.state
				m.enterStep(wizAccountName)
				return m, m.input.Init()
			}
		}

	case wizCookieRetry:
		m.confirm.Update(msg)
		if m.confirm.cancelled || (m.confirm.done && !m.confirm.value) {
			m.cancelled = true
			return m, nil
		}
		if m.confirm.done && m.confirm.value {
			m.enterStep(wizCookieInput)
			return m, m.input.Init()
		}

	case wizAccountName:
		cmd := m.input.Update(msg)
		if m.input.cancelled {
			m.cancelled = true
			return m, nil
		}
		if m.input.done {
			acct := config.Account{
				Name:   strings.TrimSpace(m.input.ti.Value()),
				Cookie: m.cookie,
			}
			m.cfg.Accounts = append(m.cfg.Accounts, acct)

			if m.mode == wizModeAddAccount {
				if err := saveConfig(m.cfg, m.cfgPath); err != nil {
					m.resultMsg = statusErr("Save failed: " + err.Error())
				} else {
					m.resultMsg = statusOK("Added account " +
						styleMenuSelected.Render(acct.Name) + " to " + m.cfgPath)
				}
				m.enterStep(wizResult)
				return m, nil
			}
			m.enterStep(wizAddAnother)
		}
		return m, cmd

	case wizAddAnother:
		m.confirm.Update(msg)
		if m.confirm.cancelled {
			m.cancelled = true
			return m, nil
		}
		if m.confirm.done {
			if m.confirm.value {
				m.cookie = ""
				m.validated = nil
				m.enterStep(wizCookieNote)
			} else {
				m.enterStep(wizSettingsNote)
			}
		}

	case wizSettingsNote:
		m.note.Update(msg)
		if m.note.cancelled {
			m.cancelled = true
			return m, nil
		}
		if m.note.done {
			m.enterStep(wizMinPoints)
			return m, m.input.Init()
		}

	case wizMinPoints:
		cmd := m.input.Update(msg)
		if m.input.cancelled {
			m.cancelled = true
			return m, nil
		}
		if m.input.done {
			n := atoiSafe(m.input.ti.Value())
			m.cfg.Defaults.MinPoints = &n
			m.enterStep(wizPauseMinutes)
			return m, m.input.Init()
		}
		return m, cmd

	case wizPauseMinutes:
		cmd := m.input.Update(msg)
		if m.input.cancelled {
			m.cancelled = true
			return m, nil
		}
		if m.input.done {
			n := atoiSafe(m.input.ti.Value())
			m.cfg.Defaults.PauseMinutes = &n
			m.enterStep(wizMaxEntries)
			return m, m.input.Init()
		}
		return m, cmd

	case wizMaxEntries:
		cmd := m.input.Update(msg)
		if m.input.cancelled {
			m.cancelled = true
			return m, nil
		}
		if m.input.done {
			n := atoiSafe(m.input.ti.Value())
			m.cfg.Defaults.MaxEntriesPerRun = &n
			m.enterStep(wizPinnedConfirm)
		}
		return m, cmd

	case wizPinnedConfirm:
		m.confirm.Update(msg)
		if m.confirm.cancelled {
			m.cancelled = true
			return m, nil
		}
		if m.confirm.done {
			v := m.confirm.value
			m.cfg.Defaults.EnterPinned = &v
			m.enterStep(wizSaveConfirm)
		}

	case wizSaveConfirm:
		m.confirm.Update(msg)
		if m.confirm.cancelled || (m.confirm.done && !m.confirm.value) {
			m.resultMsg = styleDim.Render("Setup cancelled — nothing saved.")
			m.enterStep(wizResult)
			return m, nil
		}
		if m.confirm.done && m.confirm.value {
			if err := saveConfig(m.cfg, m.cfgPath); err != nil {
				m.resultMsg = statusErr("Save failed: " + err.Error())
			} else {
				m.resultMsg = statusOK("Config saved to " + m.cfgPath)
			}
			m.enterStep(wizServiceConfirm)
		}

	case wizServiceConfirm:
		m.confirm.Update(msg)
		if m.confirm.cancelled || (m.confirm.done && !m.confirm.value) {
			m.enterStep(wizResult)
			return m, nil
		}
		if m.confirm.done && m.confirm.value {
			path, err := service.Install()
			if err != nil {
				m.resultMsg += "\n" + statusErr("Service install failed: " + err.Error())
			} else {
				m.resultMsg += "\n" + statusOK("Service installed: " + path)
			}
			m.enterStep(wizResult)
		}

	case wizResult:
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "enter", "esc", "q", " ":
				m.done = true
			}
		}
	}
	return m, nil
}

func (m wizardFlowModel) View() string {
	switch m.step {
	case wizCookieNote, wizSettingsNote:
		return m.note.View()
	case wizCookieInput, wizAccountName, wizMinPoints, wizPauseMinutes, wizMaxEntries:
		return m.input.View()
	case wizCookieValidating:
		return "\n  " + styleDim.Render("checking cookie against steamgifts.com...") + "\n"
	case wizCookieRetry, wizAddAnother, wizPinnedConfirm, wizSaveConfirm, wizServiceConfirm:
		return m.confirm.View()
	case wizResult:
		return "\n  " + m.resultMsg + "\n"
	}
	return ""
}

func (m wizardFlowModel) FooterKeys() string {
	switch m.step {
	case wizCookieNote, wizSettingsNote:
		return footerHint("enter", "continue") + "    " + footerHint("esc/q", "back")
	case wizCookieInput, wizAccountName, wizMinPoints, wizPauseMinutes, wizMaxEntries:
		return footerHint("enter", "submit") + "    " + footerHint("esc", "cancel")
	case wizCookieRetry, wizAddAnother, wizPinnedConfirm, wizSaveConfirm, wizServiceConfirm:
		return footerHint("↑↓", "choose") + "    " + footerHint("enter", "confirm") + "    " + footerHint("esc/q", "cancel")
	case wizCookieValidating:
		return ""
	case wizResult:
		return footerHint("enter", "done") + "    " + footerHint("esc/q", "back")
	}
	return ""
}

func (m wizardFlowModel) validateCookie() tea.Cmd {
	cookie := m.cookie
	ua := ""
	if m.cfg != nil {
		ua = m.cfg.Defaults.UserAgent
	}
	return func() tea.Msg {
		if ua == "" {
			ua = client.DefaultUserAgent
		}
		c, err := client.New(cookie, ua)
		if err != nil {
			return cookieValidatedMsg{err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		body, err := c.Get(ctx, "/")
		if err != nil {
			return cookieValidatedMsg{err: err}
		}
		state, _, err := sg.ParseListPage(body)
		if err != nil {
			return cookieValidatedMsg{err: err}
		}
		if state.Username == "" {
			return cookieValidatedMsg{err: errors.New("cookie invalid or expired")}
		}
		return cookieValidatedMsg{state: &state}
	}
}

func wizServiceDescription() string {
	switch service.Platform() {
	case "windows":
		return "Adds a script to your Startup folder so the bot starts\nminimized when you log in. No admin required."
	case "linux":
		return "Writes a systemd user unit and enables it.\nRuns as you, fully reversible."
	case "darwin":
		return "Writes a LaunchAgent plist so the bot starts on login.\nRuns as you, fully reversible."
	default:
		return "Installs a launcher so the bot starts automatically."
	}
}

func intRangeValidator(lo, hi int) func(string) error {
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

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func derefInt(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}

func (m wizardFlowModel) Breadcrumb() string {
	if m.mode == wizModeAddAccount {
		return "add account"
	}
	return "setup wizard"
}

func (m wizardFlowModel) FooterStatus() string { return "" }

func (m wizardFlowModel) Done() bool {
	return m.done || m.cancelled
}
