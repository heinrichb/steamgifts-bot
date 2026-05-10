package cli

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newAccountsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "Add, list, or remove configured accounts",
	}
	cmd.AddCommand(newAccountsListCmd(), newAccountsAddCmd(), newAccountsRemoveCmd())
	return cmd
}

func newAccountsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured accounts (cookies redacted)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			configPath, _ := cmd.Flags().GetString("config")
			cfg, path, err := loadConfig(configPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return errors.New("no config found — run `steamgifts-bot setup` to create one")
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "config: %s\n\n", path)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tCOOKIE\tFILTERS")
			for _, a := range cfg.Accounts {
				filters := "(global defaults)"
				if len(a.Filters) > 0 {
					filters = fmt.Sprint(a.Filters)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", a.Name, redact(a.Cookie), filters)
			}
			return tw.Flush()
		},
	}
}

func newAccountsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new account interactively (launches the cookie wizard)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return addAccountInteractive(cmd)
		},
	}
}

func newAccountsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a configured account by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			configPath, _ := cmd.Flags().GetString("config")
			cfg, path, err := loadConfig(configPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return errors.New("no config found — run `steamgifts-bot setup` to create one")
				}
				return err
			}
			out := cfg.Accounts[:0]
			removed := false
			for _, a := range cfg.Accounts {
				if a.Name == name {
					removed = true
					continue
				}
				out = append(out, a)
			}
			if !removed {
				return fmt.Errorf("account %q not found", name)
			}
			cfg.Accounts = out
			if err := saveConfig(cfg, path); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ removed account %q from %s\n", name, path)
			return nil
		},
	}
}

func redact(cookie string) string {
	if len(cookie) <= 8 {
		return "********"
	}
	return cookie[:4] + "…" + cookie[len(cookie)-4:]
}
