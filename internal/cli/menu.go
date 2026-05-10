package cli

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/heinrichb/steamgifts-bot/internal/config"
	"github.com/heinrichb/steamgifts-bot/internal/update"
)

const (
	menuRun              = "run"
	menuRunDryRun        = "dry-run"
	menuCheck            = "check"
	menuAddAccount       = "add-account"
	menuBackup           = "backup"
	menuServiceInstall   = "install-service"
	menuServiceUninstall = "uninstall-service"
	menuViewLogs         = "view-logs"
	menuUpdate           = "update"
	menuSetup            = "setup"
	menuQuit             = "quit"
)

type menuItem struct {
	label string
	desc  string
	key   string
}

type menuModel struct {
	items  []menuItem
	cursor int
	chosen string
}

func newMenuModelWithState(cfg *config.Config, updateResult *update.Result, svcInstalled, svcActive bool) menuModel {
	var items []menuItem

	if svcActive {
		items = append(items, menuItem{"View service logs", "tail the background service log", menuViewLogs})
	} else {
		items = append(items, menuItem{"Run bot", "start scanning and entering giveaways", menuRun})
		items = append(items, menuItem{"Dry run", "scan without entering any giveaways", menuRunDryRun})
	}

	items = append(items, menuItem{"Check accounts", "validate cookies and show points", menuCheck})
	items = append(items, menuItem{"Add an account", "capture a new cookie interactively", menuAddAccount})
	items = append(items, menuItem{"Back up config", "create a backup archive", menuBackup})

	if svcInstalled {
		items = append(items, menuItem{"Uninstall service", "remove the background service", menuServiceUninstall})
	} else {
		items = append(items, menuItem{"Install service", "set up as a background service", menuServiceInstall})
	}

	if updateResult != nil && updateResult.Available {
		items = append(items, menuItem{"Update to " + updateResult.LatestVersion, "download and apply the update", menuUpdate})
	}

	items = append(items,
		menuItem{"Setup wizard", "reconfigure from scratch", menuSetup},
		menuItem{"Quit", "exit the application", menuQuit},
	)

	return menuModel{
		items: items,
	}
}

func (m menuModel) Init() tea.Cmd { return nil }

func (m menuModel) Update(msg tea.Msg) (menuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "w":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "s":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = len(m.items) - 1
		case "enter":
			if m.cursor >= 0 && m.cursor < len(m.items) {
				m.chosen = m.items[m.cursor].key
			}
		case "q", "esc":
			m.chosen = menuQuit
		}
	}
	return m, nil
}

func (m menuModel) View() string {
	var b strings.Builder

	for i, item := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = styleMenuCursor.Render("▸ ")
		}

		label := styleMenuItem.Render(item.label)
		if i == m.cursor {
			label = styleMenuSelected.Render(item.label)
		}

		desc := ""
		if i == m.cursor {
			desc = "  " + styleMenuDesc.Render(item.desc)
		}

		b.WriteString(cursor + label + desc + "\n")
	}

	return b.String()
}
