package cli

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heinrichb/steamgifts-bot/internal/config"
	"github.com/heinrichb/steamgifts-bot/internal/service"
	"github.com/heinrichb/steamgifts-bot/internal/update"
)

var asciiLogo = []string{
	"  ___ _                  ___ _  __ _        ___      _   ",
	" / __| |_ ___ __ _ _ __ / __(_)/ _| |_ ___ | _ ) ___| |_ ",
	" \\__ \\  _/ -_) _` | '  \\ (_ | |  _|  _(_-< | _ \\/ _ \\  _|",
	" |___/\\__\\___\\__,_|_|_|_\\___|_|_|  \\__/__/ |___/\\___/\\__|",
}

type loadStep struct {
	label string
	done  bool
}

type splashModel struct {
	version string
	steps   []loadStep
	current int
	width   int
	height  int
	ready   bool

	cfg          *config.Config
	cfgPath      string
	updateResult *update.Result
	svcInstalled bool
	svcActive    bool
	loadErr      error

	showSplash bool
	startTime  time.Time

	displayPct float64
	frame      int
	loaded     bool
	rng        *rand.Rand

	updating    bool
	updatePhase string
	updatePct   float64
	updateDone  bool
	updateErr   error
	updateCh    chan tea.Msg
}

type splashTickMsg struct{}
type configLoadedMsg struct {
	cfg  *config.Config
	path string
	err  error
}
type serviceCheckedMsg struct {
	installed bool
	active    bool
}
type updateCheckedMsg struct {
	result *update.Result
}
type splashUpdateProgressMsg struct {
	phase string
	pct   float64
}
type splashUpdateDoneMsg struct{ path string }
type splashUpdateErrorMsg struct{ err error }

func newSplashModel(version string, configPath string, showSplash bool) splashModel {
	return splashModel{
		version: version,
		steps: []loadStep{
			{label: "loading configuration"},
			{label: "checking service status"},
			{label: "checking for updates"},
		},
		showSplash: showSplash,
		startTime:  time.Now(),
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		updateCh:   make(chan tea.Msg, 64),
	}
}

func (m splashModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadConfig(),
		m.checkService(),
		m.checkUpdate(),
		splashTick(),
	)
}

func splashTick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg { return splashTickMsg{} })
}

func (m splashModel) loadConfig() tea.Cmd {
	return func() tea.Msg {
		cfg, path, err := loadConfig("")
		return configLoadedMsg{cfg: cfg, path: path, err: err}
	}
}

func (m splashModel) checkService() tea.Cmd {
	return func() tea.Msg {
		installed := service.IsInstalled()
		active := false
		if installed {
			active = service.IsActive()
		}
		return serviceCheckedMsg{installed: installed, active: active}
	}
}

func (m splashModel) checkUpdate() tea.Cmd {
	return func() tea.Msg {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if r := update.Latest(); r != nil {
				return updateCheckedMsg{result: r}
			}
			time.Sleep(100 * time.Millisecond)
		}
		return updateCheckedMsg{result: update.Latest()}
	}
}

func waitSplashUpdate(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

func (m splashModel) startAutoUpdate() tea.Cmd {
	go func() {
		path, err := update.ApplyWithProgress(context.Background(), m.updateResult.Release, func(phase string, pct float64) {
			m.updateCh <- splashUpdateProgressMsg{phase, pct}
		})
		if err != nil {
			m.updateCh <- splashUpdateErrorMsg{err}
		} else {
			m.updateCh <- splashUpdateDoneMsg{path}
		}
	}()
	return waitSplashUpdate(m.updateCh)
}

func (m splashModel) shouldAutoUpdate() bool {
	return m.showSplash && !m.updating && !m.updateDone && m.updateErr == nil &&
		m.cfg != nil && m.cfg.AutoUpdateEnabled() &&
		m.updateResult != nil && m.updateResult.Available
}

func (m splashModel) Update(msg tea.Msg) (splashModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.ready = true
		case "enter", " ":
			if m.updateDone || m.updateErr != nil {
				m.ready = true
			} else if m.loaded && !m.updating {
				m.ready = true
			}
		}
	case configLoadedMsg:
		m.cfg = msg.cfg
		m.cfgPath = msg.path
		m.loadErr = msg.err
		m.steps[0].done = true
		m.advanceCurrent()
	case serviceCheckedMsg:
		m.svcInstalled = msg.installed
		m.svcActive = msg.active
		m.steps[1].done = true
		m.advanceCurrent()
	case updateCheckedMsg:
		m.updateResult = msg.result
		m.steps[2].done = true
		m.advanceCurrent()
	case splashUpdateProgressMsg:
		m.updatePhase = msg.phase
		m.updatePct = msg.pct
		return m, waitSplashUpdate(m.updateCh)
	case splashUpdateDoneMsg:
		m.updateDone = true
		m.updatePhase = "done"
		m.displayPct = 1.0
		return m, nil
	case splashUpdateErrorMsg:
		m.updateErr = msg.err
		m.updatePhase = "error"
		return m, nil
	case splashTickMsg:
		m.frame++

		if m.updating {
			var target float64
			switch m.updatePhase {
			case "downloading":
				target = m.updatePct
			case "extracting":
				target = 0.95
			case "installing":
				target = 0.98
			case "done":
				target = 1.0
			default:
				target = m.displayPct
			}
			if m.displayPct < target {
				gap := target - m.displayPct
				m.displayPct += gap * 0.15
				if target-m.displayPct < 0.005 {
					m.displayPct = target
				}
			}
		} else {
			target := m.targetPct()
			if m.displayPct < target {
				gap := target - m.displayPct
				m.displayPct += gap * 0.12
				if target-m.displayPct < 0.005 {
					m.displayPct = target
				}
			}

			if m.allDone() && m.displayPct >= 0.99 {
				m.displayPct = 1.0
				m.loaded = true
			}

			if m.loaded && m.shouldAutoUpdate() {
				m.updating = true
				m.updatePhase = "downloading"
				m.displayPct = 0
				return m, tea.Batch(splashTick(), m.startAutoUpdate())
			}
		}

		if m.loaded && !m.updating {
			if !m.showSplash {
				m.ready = true
			}
			return m, nil
		}

		if m.updateDone || m.updateErr != nil {
			return m, nil
		}

		return m, splashTick()
	}
	return m, nil
}

func (m *splashModel) advanceCurrent() {
	for i, s := range m.steps {
		if !s.done {
			m.current = i
			return
		}
	}
	m.current = len(m.steps)
}

func (m splashModel) allDone() bool {
	for _, s := range m.steps {
		if !s.done {
			return false
		}
	}
	return true
}

func (m splashModel) targetPct() float64 {
	done := 0
	for _, s := range m.steps {
		if s.done {
			done++
		}
	}
	return float64(done) / float64(len(m.steps))
}

func (m splashModel) View() string {
	if !m.showSplash {
		return m.renderMinimal()
	}

	var b strings.Builder

	padTop := (m.height - 14) / 3
	if padTop < 1 {
		padTop = 1
	}
	for i := 0; i < padTop; i++ {
		b.WriteString("\n")
	}

	for _, line := range asciiLogo {
		row := m.renderLogoLine(line)
		centered := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, row)
		b.WriteString(centered)
		b.WriteString("\n")
	}

	var versionLine string
	if m.updateDone {
		versionLine = styleDim.Render("v"+m.version+" → ") + styleOKBold.Render(m.updateResult.LatestVersion)
	} else {
		versionLine = styleDim.Render("v" + m.version)
	}
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, versionLine))
	b.WriteString("\n\n")

	bar := m.renderProgressBar(40)
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, bar))
	b.WriteString("\n\n")

	var statusLabel string
	switch {
	case m.updateDone:
		statusLabel = styleOK.Render("✓ updated") + styleDim.Render(" — restart to use the new version")
	case m.updateErr != nil:
		errText := m.updateErr.Error()
		if len(errText) > 50 {
			errText = errText[:47] + "..."
		}
		statusLabel = styleErr.Render("✗ update failed: " + errText)
	case m.updating:
		statusLabel = styleProgressText.Render(m.updatePhase + " " + m.updateResult.LatestVersion + "...")
	case m.loaded:
		statusLabel = footerHint("enter", "continue")
	default:
		if m.current < len(m.steps) {
			statusLabel = styleProgressText.Render(m.steps[m.current].label + "...")
		}
	}
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, statusLabel))
	b.WriteString("\n")

	if m.updateDone || m.updateErr != nil {
		b.WriteString("\n")
		prompt := footerHint("enter", "continue")
		b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, prompt))
		b.WriteString("\n")
	}

	return b.String()
}

func (m splashModel) renderMinimal() string {
	var b strings.Builder
	b.WriteString("\n")
	title := gradientMulti("SteamGifts Bot", brandColors...)
	b.WriteString("  " + title + "  " + styleDim.Render("v"+m.version) + "\n\n")
	bar := m.renderProgressBar(30)
	b.WriteString("  " + bar + "\n")
	if !m.loaded && m.current < len(m.steps) {
		b.WriteString("  " + styleProgressText.Render(m.steps[m.current].label+"...") + "\n")
	}
	return b.String()
}

func (m splashModel) renderLogoLine(line string) string {
	scrambleChars := []rune("@#$%&*!=+~^<>/|\\{}[]()0123456789")
	speed := 1.2
	sweepLag := 3.0
	cycleFrames := 5
	logoWidth := len([]rune(asciiLogo[0]))
	sweepWidth := float64(logoWidth) * 0.25

	scrambleFront := float64(m.frame) * speed
	sweepPos := scrambleFront - sweepLag

	runes := []rune(line)
	var row string
	for i, r := range runes {
		fi := float64(i)
		t := fi / float64(max(len(runes)-1, 1))
		base := multiLerpRGB(t, brandRGBs)

		if fi > scrambleFront {
			row += " "
			continue
		}

		framesSinceReveal := int(scrambleFront-fi) / max(int(speed), 1)
		resolved := framesSinceReveal > cycleFrames
		var ch rune
		if resolved {
			ch = r
		} else {
			if r == ' ' {
				ch = ' '
			} else {
				ch = scrambleChars[m.rng.Intn(len(scrambleChars))]
			}
		}

		activation := 0.0
		if fi < sweepPos {
			activation = math.Min((sweepPos-fi)/sweepWidth, 1.0)
		}

		dimFactor := 0.2
		if !resolved {
			dimFactor = 0.35
		}
		dimmed := rgb{base.r * dimFactor, base.g * dimFactor, base.b * dimFactor}
		final := lerpRGB(dimmed, base, activation)

		dist := math.Abs(fi - sweepPos)
		if dist < sweepWidth*0.4 && activation > 0 {
			boost := 0.4 * (1.0 - dist/(sweepWidth*0.4))
			final.r = math.Min(final.r+boost*255, 255)
			final.g = math.Min(final.g+boost*255, 255)
			final.b = math.Min(final.b+boost*255, 255)
		}

		row += lipgloss.NewStyle().Foreground(lipgloss.Color(final.hex())).Render(string(ch))
	}
	return row
}

func (m splashModel) renderProgressBar(width int) string {
	pct := m.displayPct
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}

	shimmerSpeed := 0.08
	shimmerWidth := 0.4

	var bar strings.Builder
	bar.WriteString(styleDim.Render("▐"))
	for i := 0; i < width; i++ {
		if i < filled {
			t := float64(i) / float64(max(width-1, 1))

			brightness := 0.0
			if pct < 1.0 {
				shimmerPos := math.Mod(float64(m.frame)*shimmerSpeed, 1.0+shimmerWidth) - shimmerWidth/2
				dist := math.Abs(t - shimmerPos)
				if dist < shimmerWidth/2 {
					brightness = 0.3 * (1.0 - dist/(shimmerWidth/2))
				}
			}

			base := multiLerpRGB(t, brandRGBs)
			lit := rgb{
				r: math.Min(base.r+brightness*255, 255),
				g: math.Min(base.g+brightness*255, 255),
				b: math.Min(base.b+brightness*255, 255),
			}
			bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(lit.hex())).Render("█"))
		} else {
			bar.WriteString(styleProgressEmpty.Render("░"))
		}
	}
	bar.WriteString(styleDim.Render("▌"))
	bar.WriteString(" " + styleProgressText.Render(fmt.Sprintf("%d%%", int(pct*100))))
	return bar.String()
}
