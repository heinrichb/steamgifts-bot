// Package cli wires the cobra command tree together.
//
// The bare `steamgifts-bot` command is intentionally smart: if a config
// file is discoverable it runs the bot; if not, it launches the first-run
// setup wizard. This is the zero-friction path for non-technical users
// who download the .exe and double-click it.
package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			// Smart default: config present → run; otherwise launch the wizard.
			if findConfig() != "" {
				return runRun(cmd, args)
			}
			fmt.Fprintln(os.Stderr, "No config found — launching first-run setup wizard.")
			fmt.Fprintln(os.Stderr, "")
			return runSetup(cmd, args)
		},
	}

	root.PersistentFlags().StringP("config", "c", "", "path to config.yaml (default: auto-discovered)")
	root.PersistentFlags().String("log-level", "info", "log level: debug, info, warn, error")

	root.AddCommand(
		newRunCmd(),
		newSetupCmd(),
		newCheckCmd(),
		newAccountsCmd(),
		newServiceCmd(),
		newVersionCmd(version, commit, date),
	)
	return root
}

// errExitSilent is returned by subcommands that have already printed a
// human-readable error and just want a non-zero exit.
var errExitSilent = errors.New("")
