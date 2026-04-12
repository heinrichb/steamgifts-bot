package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/heinrichb/steamgifts-bot/internal/client"
	"github.com/heinrichb/steamgifts-bot/internal/config"
	logpkg "github.com/heinrichb/steamgifts-bot/internal/log"
	"github.com/heinrichb/steamgifts-bot/internal/ratelimit"
	sg "github.com/heinrichb/steamgifts-bot/internal/steamgifts"
)

var (
	checkOK    = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Bold(true)
	checkFail  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	checkLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true)
)

func newCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Validate config + cookies, show account points, then exit",
		Long: `Reads the config file, validates it, and pings steamgifts.com
once per account to confirm each cookie still works. Prints a tidy
summary table and exits non-zero if anything fails.

This is what the wizard runs after you paste a cookie, and what you'd
hook up to a healthcheck or smoke test.`,
		RunE: runCheck,
	}
}

func runCheck(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	levelStr, _ := cmd.Flags().GetString("log-level")
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
	logger.Info("config loaded", "path", path)

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw,
		checkLabel.Render("ACCOUNT")+"\t"+
			checkLabel.Render("USERNAME")+"\t"+
			checkLabel.Render("POINTS")+"\t"+
			checkLabel.Render("LEVEL")+"\t"+
			checkLabel.Render("STATUS"))

	anyFailed := false
	for i, acct := range cfg.Accounts {
		settings := cfg.Resolved(i)
		st, err := probeAccount(ctx, acct, settings)
		switch {
		case err != nil:
			anyFailed = true
			fmt.Fprintf(tw, "%s\t-\t-\t-\t%s\n",
				acct.Name, checkFail.Render("FAIL: "+err.Error()))
		default:
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n",
				acct.Name, st.Username, st.Points, st.Level, checkOK.Render("OK"))
		}
	}
	_ = tw.Flush()

	if anyFailed {
		return errExitSilent
	}
	return nil
}

// probeAccount fetches the front page once and returns the parsed account state.
// Exposed inside the package so the wizard can reuse it for live cookie validation.
func probeAccount(ctx context.Context, acct config.Account, settings config.AccountSettings) (sg.AccountState, error) {
	c, err := client.New(acct.Cookie, settings.UserAgent,
		client.WithLimiter(ratelimit.New(0, 0)),
	)
	if err != nil {
		return sg.AccountState{}, err
	}
	body, err := c.Get(ctx, "/")
	if err != nil {
		return sg.AccountState{}, err
	}
	state, _, err := sg.ParseListPage(body)
	if err != nil {
		return sg.AccountState{}, err
	}
	if state.Username == "" {
		return state, errors.New("signed-in username not found — cookie likely invalid")
	}
	return state, nil
}
