package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/heinrichb/steamgifts-bot/internal/config"
	"github.com/heinrichb/steamgifts-bot/internal/wizard"
)

func newSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Run the interactive first-run setup wizard",
		Long: `Walks you through:

  1. Capturing your steamgifts.com PHPSESSID cookie (with browser help)
  2. Setting points / pause / filter preferences
  3. Adding additional accounts (optional)
  4. Saving the config file
  5. Optionally installing a background service so the bot starts on login

The wizard is fully re-runnable: it loads your existing config (if any),
lets you edit, and asks before saving over it.`,
		RunE: runSetup,
	}
}

func runSetup(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	cfg, existingPath, err := loadConfig(configPath)
	switch {
	case err == nil:
		fmt.Fprintf(cmd.OutOrStdout(), "Loaded existing config from %s — you can edit it.\n\n", existingPath)
	case errors.Is(err, os.ErrNotExist):
		d := config.Defaults()
		cfg = &d
	default:
		return err
	}

	savePath := existingPath
	if savePath == "" {
		savePath = defaultSavePath()
	}

	out, err := wizard.Run(cmd.Context(), wizard.Options{
		Config:   cfg,
		SavePath: savePath,
	})
	if err != nil {
		return err
	}
	if out.Saved {
		fmt.Fprintf(cmd.OutOrStdout(), "\n✓ Config saved to %s\n", out.Path)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\nSetup cancelled — nothing saved.")
	}
	return nil
}
