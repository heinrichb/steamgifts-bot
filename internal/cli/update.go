package cli

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/heinrichb/steamgifts-bot/internal/update"
)

func newUpdateCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Check for and apply available updates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "Checking for updates...")
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			update.Check(ctx, slog.Default(), version)
			result := update.Latest()
			if result == nil || !result.Available {
				fmt.Fprintln(cmd.OutOrStdout(), "You are running the latest version.")
				return nil
			}
			return runUpdate(cmd, result)
		},
	}
}
