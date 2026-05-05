package cli

import (
	"fmt"
	"time"

	"github.com/charmbracelet/huh"

	"github.com/heinrichb/steamgifts-bot/internal/service"
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
	menuUpdate           = "update"
	menuSetup            = "setup"
	menuQuit             = "quit"
)

// waitForUpdateCheck gives the background update check a short window to
// finish so the menu can reflect whether an update is available.
func waitForUpdateCheck() *update.Result {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r := update.Latest(); r != nil {
			return r
		}
		time.Sleep(100 * time.Millisecond)
	}
	return update.Latest()
}

// showMenu presents an interactive menu so users who double-click the exe
// (or just run `steamgifts-bot` with a config present) can navigate without
// knowing CLI flags.
func showMenu(updateResult *update.Result) (string, error) {
	var serviceOption huh.Option[string]
	if service.IsInstalled() {
		serviceOption = huh.NewOption("Uninstall background service", menuServiceUninstall)
	} else {
		serviceOption = huh.NewOption("Install as background service", menuServiceInstall)
	}

	description := "What would you like to do?"
	if updateResult != nil && updateResult.Available {
		description = fmt.Sprintf("⚡ Update available: %s → %s", updateResult.CurrentVersion, updateResult.LatestVersion)
	}

	opts := []huh.Option[string]{
		huh.NewOption("Run bot", menuRun),
		huh.NewOption("Run bot (dry-run — no entries submitted)", menuRunDryRun),
		huh.NewOption("Check accounts (validate cookies + show points)", menuCheck),
		huh.NewOption("Add an account", menuAddAccount),
		huh.NewOption("Back up config + state", menuBackup),
		serviceOption,
	}
	if updateResult != nil && updateResult.Available {
		label := fmt.Sprintf("Update to %s", updateResult.LatestVersion)
		opts = append(opts, huh.NewOption(label, menuUpdate))
	}
	opts = append(opts,
		huh.NewOption("Re-run setup wizard", menuSetup),
		huh.NewOption("Quit", menuQuit),
	)

	choice := menuRun
	err := huh.NewSelect[string]().
		Title("steamgifts-bot").
		Description(description).
		Options(opts...).
		Value(&choice).
		WithTheme(huh.ThemeCharm()).
		Run()
	return choice, err
}
