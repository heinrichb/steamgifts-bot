package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/heinrichb/steamgifts-bot/internal/account"
	logpkg "github.com/heinrichb/steamgifts-bot/internal/log"
	metrics "github.com/heinrichb/steamgifts-bot/internal/metrics"
	"github.com/heinrichb/steamgifts-bot/internal/notify"
	"github.com/heinrichb/steamgifts-bot/internal/state"
	"github.com/heinrichb/steamgifts-bot/internal/update"
	"github.com/heinrichb/steamgifts-bot/internal/web"
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
	cmd.Flags().String("dashboard-addr", "", "address for web dashboard (e.g. :8080). Disabled if empty.")
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
	dashboardAddr, _ := cmd.Flags().GetString("dashboard-addr")

	// Determine log file path: next to config, or a default location.
	logFilePath := ""
	if p := findConfig(); p != "" {
		logFilePath = filepath.Join(filepath.Dir(p), "steamgifts-bot.log")
	} else if dir, err := os.UserConfigDir(); err == nil {
		logFilePath = filepath.Join(dir, "steamgifts-bot", "steamgifts-bot.log")
	}

	var logger *slog.Logger
	if logFilePath != "" {
		l, cleanup, err := logpkg.NewWithFile(levelStr, logFormat, logFilePath)
		if err != nil {
			// Fall back to console-only if file logging fails.
			l2, _ := logpkg.New(os.Stderr, levelStr, logFormat)
			l2.Warn("failed to open log file, using console only", "path", logFilePath, "err", err)
			logger = l2
		} else {
			defer cleanup()
			logger = l
			logger.Info("logging to file", "path", logFilePath)
		}
	} else {
		l, lerr := logpkg.New(os.Stderr, levelStr, logFormat)
		if lerr != nil {
			return lerr
		}
		logger = l
	}

	if metricsAddr != "" {
		logger.Info("starting metrics server", "addr", metricsAddr)
		metricsCtx, metricsCancel := context.WithCancel(context.Background())
		defer metricsCancel()
		go func() {
			if err := metrics.Serve(metricsCtx, metricsAddr); err != nil {
				logger.Error("metrics server failed", "err", err)
			}
		}()
	}

	sighup := sighupChan()

	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n\n",
		gradientMulti("SteamGifts Bot", brandColors...),
		styleDim.Render("v"+buildVersion))

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

		if dashboardAddr != "" {
			logger.Info("starting dashboard", "addr", dashboardAddr)
			go func() {
				if err := web.Serve(ctx, dashboardAddr, orch); err != nil {
					logger.Error("dashboard server failed", "err", err)
				}
			}()
		}

		var updateCh chan string
		if cfg.AutoUpdateEnabled() {
			updateCh = make(chan string, 1)
			go func() {
				interval := cfg.UpdateCheckInterval()
				logger.Info("auto-update enabled", "interval", interval)

				select {
				case <-ctx.Done():
					return
				case <-time.After(30 * time.Second):
				}

				for {
					update.Check(ctx, logger, buildVersion)
					if r := update.Latest(); r != nil && r.Available {
						logger.Info("update available, applying",
							"current", buildVersion, "latest", r.LatestVersion)
						if _, err := update.Apply(ctx, r.Release); err != nil {
							logger.Error("auto-update failed", "err", err)
						} else {
							logger.Info("update applied, exiting for restart",
								"version", r.LatestVersion)
							updateCh <- r.LatestVersion
							return
						}
					}

					select {
					case <-ctx.Done():
						return
					case <-time.After(interval):
					}
				}
			}()
		}

		// Headless mode: run until SIGINT/SIGTERM, SIGHUP, or auto-update.
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
		case ver := <-updateCh:
			logger.Info("shutting down after update", "version", ver)
			stop()
			<-done
			fmt.Fprintf(os.Stdout, "Updated to %s. Restart to use the new version.\n", ver)
			return nil
		}
	}
}
