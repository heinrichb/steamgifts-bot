package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/heinrichb/steamgifts-bot/internal/client"
	"github.com/heinrichb/steamgifts-bot/internal/config"
	logpkg "github.com/heinrichb/steamgifts-bot/internal/log"
	"github.com/heinrichb/steamgifts-bot/internal/ratelimit"
	sg "github.com/heinrichb/steamgifts-bot/internal/steamgifts"
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
	logFormat, _ := cmd.Flags().GetString("log-format")
	logger, err := logpkg.New(os.Stderr, levelStr, logFormat)
	if err != nil {
		return err
	}

	cfg, path, err := loadValidConfig(configPath)
	if err != nil {
		return err
	}
	logger.Info("config loaded", "path", path)

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	anyFailed := false
	for i, acct := range cfg.Accounts {
		settings := cfg.Resolved(i)
		st, perr := probeAccount(ctx, acct, settings)
		if perr != nil {
			anyFailed = true
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s\n",
				acct.Name, styleErrBold.Render("FAIL"), perr.Error())
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  %s  points=%d  level=%d\n",
				acct.Name, st.Username, styleOKBold.Render("OK"), st.Points, st.Level)
		}
	}

	if anyFailed {
		return errExitSilent
	}
	return nil
}

type checkResult struct {
	name     string
	username string
	points   int
	level    int
	err      error
}

type checkResultMsg checkResult
type checkDoneMsg struct{}

type checkModel struct {
	cfg     *config.Config
	results []checkResult
	done    bool
	pending int
	back    bool
}

func newCheckModel(cfg *config.Config) checkModel {
	return checkModel{
		cfg:     cfg,
		pending: len(cfg.Accounts),
	}
}

func (m checkModel) Init() tea.Cmd {
	if m.cfg == nil || len(m.cfg.Accounts) == 0 {
		return func() tea.Msg { return checkDoneMsg{} }
	}

	var cmds []tea.Cmd
	for i, acct := range m.cfg.Accounts {
		acct := acct
		settings := m.cfg.Resolved(i)
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			st, err := probeAccount(ctx, acct, settings)
			return checkResultMsg(checkResult{
				name:     acct.Name,
				username: st.Username,
				points:   st.Points,
				level:    st.Level,
				err:      err,
			})
		})
	}
	return tea.Batch(cmds...)
}

func (m checkModel) Update(msg tea.Msg) (appPage, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" || msg.String() == "q" {
			m.back = true
			return m, nil
		}
	case checkResultMsg:
		m.results = append(m.results, checkResult(msg))
		m.pending--
		if m.pending <= 0 {
			m.done = true
		}
	case checkDoneMsg:
		m.done = true
	}
	return m, nil
}

func (m checkModel) Breadcrumb() string { return "check accounts" }

func (m checkModel) FooterKeys() string {
	return footerHint("esc/q", "back")
}

func (m checkModel) FooterStatus() string { return "" }

func (m checkModel) Done() bool { return m.back }

func (m checkModel) View() string {
	var b strings.Builder

	if len(m.results) == 0 && !m.done {
		b.WriteString("\n  " + styleDim.Render("checking accounts...") + "\n")
		return b.String()
	}

	if m.cfg == nil || len(m.cfg.Accounts) == 0 {
		b.WriteString("\n  " + styleWarn.Render("no accounts configured") + "\n")
		return b.String()
	}

	var rows [][]string
	for _, r := range m.results {
		status := styleOKBold.Render("OK")
		username := r.username
		points := fmt.Sprintf("%d", r.points)
		level := fmt.Sprintf("%d", r.level)
		if r.err != nil {
			status = styleErrBold.Render("FAIL")
			username = "-"
			points = "-"
			level = "-"
		}
		rows = append(rows, []string{r.name, username, points, level, status})
	}

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(tableBorder).
		Headers("ACCOUNT", "USERNAME", "POINTS", "LEVEL", "STATUS").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return tableHeader
			}
			return tableCell
		})

	b.WriteString("\n")
	b.WriteString(t.String())
	b.WriteString("\n")

	if !m.done {
		b.WriteString("\n  " + styleDim.Render(fmt.Sprintf("  checking %d remaining...", m.pending)) + "\n")
	} else {
		anyFailed := false
		for _, r := range m.results {
			if r.err != nil {
				anyFailed = true
				break
			}
		}
		if anyFailed {
			b.WriteString("\n  " + styleErrBold.Render("some accounts failed") + "\n")
		} else {
			b.WriteString("\n  " + styleOKBold.Render("all accounts OK") + "\n")
		}
	}

	return b.String()
}

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
