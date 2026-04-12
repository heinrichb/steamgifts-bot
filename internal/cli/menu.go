package cli

import (
	"github.com/charmbracelet/huh"
)

const (
	menuRun             = "run"
	menuRunDryRun       = "dry-run"
	menuCheck           = "check"
	menuAddAccount      = "add-account"
	menuServiceInstall  = "install-service"
	menuServiceStatus   = "service-status"
	menuSetup           = "setup"
	menuQuit            = "quit"
)

// showMenu presents an interactive menu so users who double-click the exe
// (or just run `steamgifts-bot` with a config present) can navigate without
// knowing CLI flags.
func showMenu() (string, error) {
	choice := menuRun
	err := huh.NewSelect[string]().
		Title("steamgifts-bot").
		Description("What would you like to do?").
		Options(
			huh.NewOption("Run bot", menuRun),
			huh.NewOption("Run bot (dry-run — no entries submitted)", menuRunDryRun),
			huh.NewOption("Check accounts (validate cookies + show points)", menuCheck),
			huh.NewOption("Add an account", menuAddAccount),
			huh.NewOption("Install as background service", menuServiceInstall),
			huh.NewOption("Background service status", menuServiceStatus),
			huh.NewOption("Re-run setup wizard", menuSetup),
			huh.NewOption("Quit", menuQuit),
		).
		Value(&choice).
		WithTheme(huh.ThemeCharm()).
		Run()
	return choice, err
}
