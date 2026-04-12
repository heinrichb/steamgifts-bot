package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/heinrichb/steamgifts-bot/internal/account"
	logpkg "github.com/heinrichb/steamgifts-bot/internal/log"
	metricspkg "github.com/heinrichb/steamgifts-bot/internal/metrics"
	"github.com/heinrichb/steamgifts-bot/internal/notify"
	"github.com/heinrichb/steamgifts-bot/internal/state"
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
	cmd.Flags().String("state-file", "", "path to state.json (default: beside config.yml)")
	cmd.Flags().String("metrics-addr", "", "address for Prometheus /metrics (e.g. :9090). Disabled if empty.")
	return cmd
}

// runBotFromMenu is called by the interactive menu with an explicit dryRun
// flag, since the run subcommand's flags aren't registered on the root cmd.
func runBotFromMenu(cmd *cobra.Command, dryRun bool) error {
	return runBot(cmd, dryRun, false, false)
}

func runRun(cmd *cobra.Command, _ []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	once, _ := cmd.Flags().GetBool("once")
	tui, _ := cmd.Flags().GetBool("tui")
	return runBot(cmd, dryRun, once, tui)
}

func runBot(cmd *cobra.Command, dryRun, once, tui bool) error {
	configPath, _ := cmd.Flags().GetString("config")
	statePath, _ := cmd.Flags().GetString("state-file")
	levelStr, _ := cmd.Flags().GetString("log-level")
	logFormat, _ := cmd.Flags().GetString("log-format")
	metricsAddr, _ := cmd.Flags().GetString("metrics-addr")

	logger, err := logpkg.New(os.Stderr, levelStr, logFormat)
	if err != nil {
		return err
	}

	if metricsAddr != "" {
		logger.Info("starting metrics server", "addr", metricsAddr)
		metricsCtx, metricsCancel := context.WithCancel(context.Background())
		defer metricsCancel()
		go func() {
			if err := metricspkg.Serve(metricsCtx, metricsAddr); err != nil {
				logger.Error("metrics server failed", "err", err)
			}
		}()
	}

	sighup := sighupChan()

	for {
		cfg, path, err := loadValidConfig(configPath)
		if err != nil {
			return err
		}

		if statePath == "" {
			statePath = state.DefaultPathFor(path)
		}
		store, err := state.Load(statePath)
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}
		notif := notify.New(cfg.DiscordWebhookURL, cfg.TelegramBotToken, cfg.TelegramChatID)
		logger.Info("loaded config",
			"path", path,
			"state", store.Path(),
			"accounts", len(cfg.Accounts),
			"dry_run", dryRun,
			"notifications", notif.Enabled(),
		)

		orch, err := account.Build(cfg, logger, store, notif, dryRun)
		if err != nil {
			return err
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

		if tui || once {
			var runErr error
			if tui {
				runErr = runTUI(ctx, orch, once)
			} else {
				runErr = orch.Run(ctx, once)
			}
			stop()
			return runErr
		}

		// Headless mode: run until SIGINT/SIGTERM or SIGHUP.
		done := make(chan error, 1)
		go func() { done <- orch.Run(ctx, false) }()

		select {
		case err := <-done:
			stop()
			return err
		case <-sighup:
			logger.Info("SIGHUP received — reloading config")
			stop()   // cancel current orchestrator
			<-done   // wait for runners to finish
			continue // loop back to reload
		}
	}
}
