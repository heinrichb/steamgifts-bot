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

	"github.com/spf13/cobra"

	"github.com/heinrichb/steamgifts-bot/internal/config"
	"github.com/heinrichb/steamgifts-bot/internal/service"
	"github.com/heinrichb/steamgifts-bot/internal/update"
	"github.com/heinrichb/steamgifts-bot/internal/wizard"
)

// NewRootCmd builds the cobra command tree. Version metadata is injected
// from main.go (which gets it from -ldflags at build time).
func NewRootCmd(version, commit, date string) *cobra.Command {
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
			go update.Check(context.Background(), slog.Default(), version)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if findConfig() == "" {
				fmt.Fprintln(os.Stderr, "No config found — launching first-run setup wizard.")
				fmt.Fprintln(os.Stderr, "")
				return runSetup(cmd, args)
			}
			return runMenu(cmd, args)
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
		newVersionCmd(version, commit, date),
	)
	return root
}

// runMenu shows the interactive menu and dispatches the chosen action.
func runMenu(cmd *cobra.Command, args []string) error {
	choice, err := showMenu()
	if err != nil {
		return err
	}
	switch choice {
	case menuRun:
		return runBotFromMenu(cmd, false)
	case menuRunDryRun:
		return runBotFromMenu(cmd, true)
	case menuCheck:
		return runCheck(cmd, args)
	case menuAddAccount:
		return addAccountInteractive(cmd)
	case menuBackup:
		return newBackupCreateCmd().RunE(cmd, nil)
	case menuServiceInstall:
		path, serr := service.Install()
		if serr != nil {
			return serr
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ installed: %s\n", path)
		return nil
	case menuServiceStatus:
		st, serr := service.Status()
		if serr != nil {
			return serr
		}
		fmt.Fprintln(cmd.OutOrStdout(), st)
		return nil
	case menuSetup:
		return runSetup(cmd, args)
	case menuQuit:
		return nil
	default:
		return nil
	}
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

// errExitSilent is returned by subcommands that have already printed a
// human-readable error and just want a non-zero exit.
var errExitSilent = errors.New("")
