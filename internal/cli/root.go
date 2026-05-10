// Package cli wires the cobra command tree together.
//
// The bare `steamgifts-bot` command is intentionally smart: if a config
// file is discoverable it runs the bot; if not, it launches the first-run
// setup wizard. This is the zero-friction path for non-technical users
// who download the .exe and double-click it.
package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/heinrichb/steamgifts-bot/internal/config"
	"github.com/heinrichb/steamgifts-bot/internal/update"
	"github.com/heinrichb/steamgifts-bot/internal/wizard"
)

const updateAppliedMsg = "✓ Updated. Please restart the application to use the new version."

var buildVersion string

func init() {
	cobra.MousetrapHelpText = ""
}

func NewRootCmd(version, commit, date string) *cobra.Command {
	buildVersion = version
	root := &cobra.Command{
		Use:   "steamgifts-bot",
		Short: "Multi-account giveaway bot for steamgifts.com",
		Long: `steamgifts-bot is a small, fast, multi-account bot for
https://www.steamgifts.com.

If you have a config file already, just run:

  steamgifts-bot

If this is your first time, run:

  steamgifts-bot setup

…and the wizard will walk you through capturing your cookie and saving a
config file. From there you can run the bot directly, install it as a
background service, or run it in Docker.`,
		Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			update.CleanupOldBinary()
			go update.Check(context.Background(), slog.Default(), version)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if findConfig() == "" {
				fmt.Fprintln(os.Stderr, "No config found — launching first-run setup wizard.")
				fmt.Fprintln(os.Stderr, "")
				return runSetup(cmd, args)
			}
			return runMenu(cmd, version)
		},
	}

	root.PersistentFlags().StringP("config", "c", "", "path to config.yml (default: auto-discovered)")
	root.PersistentFlags().String("log-level", "info", "log level: debug, info, warn, error")
	root.PersistentFlags().String("log-format", "auto", "log format: auto, text, json")

	root.AddCommand(
		newRunCmd(),
		newSetupCmd(),
		newCheckCmd(),
		newAccountsCmd(),
		newServiceCmd(),
		newBackupCmd(),
		newUpdateCmd(version),
		newVersionCmd(version, commit, date),
	)
	return root
}

func runMenu(cmd *cobra.Command, version string) error {
	configPath, _ := cmd.Flags().GetString("config")

	showSplash := true
	if cfg, _, err := loadConfig(configPath); err == nil && cfg != nil {
		showSplash = cfg.SplashScreenEnabled()
	}

	for {
		model := newAppModel(version, configPath, showSplash)
		prog := tea.NewProgram(model, tea.WithAltScreen())
		result, err := prog.Run()
		if err != nil {
			return err
		}

		app := result.(appModel)

		if app.updateApplied {
			fmt.Fprintln(cmd.OutOrStdout(), updateAppliedMsg)
			return nil
		}

		switch app.action {
		case exitNone:
			return nil
		case exitRun:
			return runBotFromMenu(cmd, false)
		case exitRunDryRun:
			return runBotFromMenu(cmd, true)
		case exitUpdate:
			return runUpdate(cmd, app.updateResult)
		}

		showSplash = false
	}
}

func runUpdate(cmd *cobra.Command, result *update.Result) error {
	if result == nil || !result.Available {
		fmt.Fprintln(cmd.OutOrStdout(), "No update available.")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s...\n", result.LatestVersion)
	path, err := update.Apply(cmd.Context(), result.Release)
	if err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✓ Updated: %s\n", path)
	fmt.Fprintln(cmd.OutOrStdout(), updateAppliedMsg)
	return nil
}

func addAccountInteractive(cmd *cobra.Command) error {
	configPath, _ := cmd.Flags().GetString("config")
	cfg, path, err := loadConfig(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if cfg == nil {
		d := config.Defaults()
		cfg = &d
		path = defaultSavePath()
	}
	acct, err := wizard.CaptureAccount(cmd.Context(), wizard.AccountInput{
		DefaultName: fmt.Sprintf("account-%d", len(cfg.Accounts)+1),
		UserAgent:   cfg.Defaults.UserAgent,
	})
	if err != nil {
		return err
	}
	cfg.Accounts = append(cfg.Accounts, acct)
	if err := saveConfig(cfg, path); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "✓ added account %q to %s\n", acct.Name, path)
	return nil
}

var errExitSilent = errors.New("")
