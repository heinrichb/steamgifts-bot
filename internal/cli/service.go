package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/heinrichb/steamgifts-bot/internal/service"
)

func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Install, uninstall, or check the background service",
		Long: `Registers steamgifts-bot to run automatically:

  * Linux:   ~/.config/systemd/user/steamgifts-bot.service
  * Windows: a .bat in your Startup folder (starts minimized on login)
  * macOS:   not yet implemented (see TODO.md)

All install paths print exactly what they did and how to undo it. No
admin elevation is required — these are user-scope installs only.`,
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "install",
			Short: "Install the bot as a background service",
			RunE: func(cmd *cobra.Command, _ []string) error {
				path, err := service.Install()
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "✓ installed: %s\n", path)
				return nil
			},
		},
		&cobra.Command{
			Use:   "uninstall",
			Short: "Remove the background service",
			RunE: func(cmd *cobra.Command, _ []string) error {
				if err := service.Uninstall(); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "✓ uninstalled")
				return nil
			},
		},
		&cobra.Command{
			Use:   "status",
			Short: "Show whether the background service is installed",
			RunE: func(cmd *cobra.Command, _ []string) error {
				st, err := service.Status()
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), st)
				return nil
			},
		},
	)
	return cmd
}
