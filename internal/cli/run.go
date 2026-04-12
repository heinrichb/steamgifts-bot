package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/heinrichb/steamgifts-bot/internal/account"
	logpkg "github.com/heinrichb/steamgifts-bot/internal/log"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the bot",
		Long: `Runs the bot for every configured account.

Each account runs independently in its own goroutine, polling the
configured filters on its own schedule. The process keeps running until
you stop it (Ctrl-C) unless --once is given.

If --tui is set, a live status dashboard is shown instead of the default
streaming logs. The dashboard is keyboard-friendly and works in any
modern terminal (cmd.exe, PowerShell, Windows Terminal, gnome-terminal).`,
		RunE: runRun,
	}
	cmd.Flags().Bool("once", false, "run a single scan cycle for each account, then exit")
	cmd.Flags().Bool("dry-run", false, "scan and log candidates without entering any giveaway")
	cmd.Flags().Bool("tui", false, "show a live status dashboard instead of streaming logs")
	return cmd
}

func runRun(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	levelStr, _ := cmd.Flags().GetString("log-level")
	once, _ := cmd.Flags().GetBool("once")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	tui, _ := cmd.Flags().GetBool("tui")

	logger, err := logpkg.New(os.Stderr, levelStr)
	if err != nil {
		return err
	}

	cfg, path, err := loadConfig(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no config found — run `steamgifts-bot setup` to create one")
		}
		return err
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config (%s): %w", path, err)
	}
	logger.Info("loaded config", "path", path, "accounts", len(cfg.Accounts), "dry_run", dryRun)

	orch, err := account.Build(cfg, logger, dryRun)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if tui {
		return runTUI(ctx, orch, once)
	}
	return orch.Run(ctx, once)
}
