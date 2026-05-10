package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heinrichb/steamgifts-bot/internal/config"
	"github.com/heinrichb/steamgifts-bot/internal/service"
	"github.com/heinrichb/steamgifts-bot/internal/update"
)

type appState int

const (
	stateSplash appState = iota
	stateMenu
	statePage
)

type exitAction int

const (
	exitNone exitAction = iota
	exitRun
	exitRunDryRun
	exitUpdate
)

type appModel struct {
	state  appState
	action exitAction

	version      string
	cfg          *config.Config
	cfgPath      string
	updateResult *update.Result
	svcInstalled bool
	svcActive    bool

	splash splashModel
	menu   menuModel
	page   appPage

	width         int
	height        int
	updateApplied bool

	cachedTitle  string
	cachedBanner lipgloss.Style
}

func newAppModel(version string, configPath string, showSplash bool) appModel {
	return appModel{
		state:        stateSplash,
		version:      version,
		splash:       newSplashModel(version, configPath, showSplash),
		cachedTitle:  gradientMulti("SteamGifts Bot", brandColors...),
		cachedBanner: gradientBorder(styleBanner, brandColors...),
	}
}

func (m appModel) Init() tea.Cmd {
	return m.splash.Init()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.action = exitNone
			return m, tea.Quit
		}
	}

	switch m.state {
	case stateSplash:
		return m.updateSplash(msg)
	case stateMenu:
		return m.updateMenu(msg)
	case statePage:
		return m.updatePage(msg)
	}
	return m, nil
}

func (m appModel) updateSplash(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.splash, cmd = m.splash.Update(msg)

	if m.splash.ready {
		if m.splash.updateDone {
			m.updateApplied = true
			m.action = exitNone
			return m, tea.Quit
		}
		m.cfg = m.splash.cfg
		m.cfgPath = m.splash.cfgPath
		m.updateResult = m.splash.updateResult
		m.svcInstalled = m.splash.svcInstalled
		m.svcActive = m.splash.svcActive
		m.state = stateMenu
		m.menu = newMenuModelWithState(m.cfg, m.updateResult, m.svcInstalled, m.svcActive)
		return m, nil
	}
	return m, cmd
}

func (m appModel) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.menu, cmd = m.menu.Update(msg)

	if m.menu.chosen != "" {
		choice := m.menu.chosen
		m.menu.chosen = ""

		switch choice {
		case menuRun:
			m.action = exitRun
			return m, tea.Quit
		case menuRunDryRun:
			m.action = exitRunDryRun
			return m, tea.Quit
		case menuUpdate:
			m.action = exitUpdate
			return m, tea.Quit
		case menuQuit:
			m.action = exitNone
			return m, tea.Quit
		case menuCheck:
			m.page = newCheckModel(m.cfg)
			m.state = statePage
			return m, m.page.Init()
		case menuViewLogs:
			lm := newLogModel(logFilePath(m.cfgPath))
			lm.width = m.width
			lm.height = m.height - headerHeight() - footerHeight()
			m.page = lm
			m.state = statePage
			return m, m.page.Init()
		case menuAddAccount:
			cfgPath := m.cfgPath
			if cfgPath == "" {
				cfgPath = defaultSavePath()
			}
			m.page = newWizardFlow(wizModeAddAccount, m.cfg, cfgPath)
			m.state = statePage
			return m, m.page.Init()
		case menuSetup:
			cfgPath := m.cfgPath
			if cfgPath == "" {
				cfgPath = defaultSavePath()
			}
			m.page = newWizardFlow(wizModeSetup, m.cfg, cfgPath)
			m.state = statePage
			return m, m.page.Init()
		case menuBackup:
			m.page = newResultPage("backup", createBackupInline(m.cfgPath))
			m.state = statePage
			return m, nil
		case menuServiceInstall:
			m.page = newResultPage("install service", doServiceInstall())
			m.state = statePage
			return m, nil
		case menuServiceUninstall:
			m.page = newResultPage("uninstall service", doServiceUninstall())
			m.state = statePage
			return m, nil
		}
	}
	return m, cmd
}

func (m appModel) updatePage(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.page, cmd = m.page.Update(msg)
	if m.page.Done() {
		return m.returnToMenu()
	}
	return m, cmd
}

func (m appModel) returnToMenu() (tea.Model, tea.Cmd) {
	if cfg, path, err := loadConfig(m.cfgPath); err == nil && cfg != nil {
		m.cfg = cfg
		m.cfgPath = path
	}
	m.svcInstalled = service.IsInstalled()
	if m.svcInstalled {
		m.svcActive = service.IsActive()
	} else {
		m.svcActive = false
	}
	m.state = stateMenu
	m.menu = newMenuModelWithState(m.cfg, m.updateResult, m.svcInstalled, m.svcActive)
	m.page = nil
	return m, nil
}

func (m appModel) View() string {
	if m.state == stateSplash {
		return m.splash.View()
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	contentHeight := m.height - headerHeight() - footerHeight()
	if contentHeight < 1 {
		contentHeight = 20
	}

	var content string
	switch m.state {
	case stateMenu:
		content = m.menu.View()
	case statePage:
		content = m.page.View()
	}

	contentBlock := constrainBlock(content, m.width, contentHeight)

	return lipgloss.JoinVertical(lipgloss.Left, header, contentBlock, footer)
}

func constrainBlock(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	if width > 0 {
		truncator := lipgloss.NewStyle().MaxWidth(width)
		for i, line := range lines {
			lines[i] = truncator.Render(line)
		}
	}
	return strings.Join(lines, "\n")
}

func (m appModel) renderHeader() string {
	title := m.cachedTitle + "  " + styleDim.Render(m.version)

	var status []string
	if m.cfg != nil {
		n := len(m.cfg.Accounts)
		label := fmt.Sprintf("%d accounts", n)
		if n == 1 {
			label = "1 account"
		}
		status = append(status, styleHeader.Render(label))
	}

	if m.svcInstalled {
		if m.svcActive {
			status = append(status, dotActive+" "+styleOK.Render("service running"))
		} else {
			status = append(status, dotStopped+" "+styleWarn.Render("service stopped"))
		}
	} else {
		status = append(status, dotNone+" "+styleDim.Render("no service"))
	}

	if m.updateResult != nil && m.updateResult.Available {
		status = append(status, styleWarn.Render("update: "+m.updateResult.CurrentVersion+" → "+m.updateResult.LatestVersion))
	}

	var breadcrumb string
	if m.state == statePage && m.page != nil {
		breadcrumb = styleDim.Render(" › ") + styleHeader.Render(m.page.Breadcrumb())
	}

	line1 := title + breadcrumb
	line2 := strings.Join(status, styleDim.Render("  ·  "))

	w := m.width - 6
	if w < 40 {
		w = 40
	}

	inner := lipgloss.NewStyle().Width(w).Render(line1 + "\n" + line2)
	box := m.cachedBanner.Render(inner)

	return box
}

func (m appModel) renderFooter() string {
	sep := styleDim.Render(strings.Repeat("─", m.width))

	var keys string
	var status string
	switch m.state {
	case stateMenu:
		keys = strings.Join([]string{
			footerHint("↑↓/ws", "navigate"),
			footerHint("enter", "select"),
			footerHint("esc/q", "quit"),
		}, "    ")
	case statePage:
		if m.page != nil {
			keys = m.page.FooterKeys()
			status = m.page.FooterStatus()
		}
	}

	footerLine := styleFooter.Render(keys)
	if status != "" {
		keysWidth := lipgloss.Width(footerLine)
		statusWidth := lipgloss.Width(status)
		gap := m.width - keysWidth - statusWidth
		if gap < 2 {
			gap = 2
		}
		footerLine = footerLine + strings.Repeat(" ", gap) + status
	}

	return sep + "\n" + footerLine
}

func headerHeight() int { return 4 }
func footerHeight() int { return 2 }

func doServiceInstall() string {
	path, err := service.Install()
	if err != nil {
		return statusErr("Install failed: " + err.Error())
	}
	return statusOK("Service installed: " + path)
}

func doServiceUninstall() string {
	if err := service.Uninstall(); err != nil {
		return statusErr("Uninstall failed: " + err.Error())
	}
	return statusOK("Service uninstalled")
}
