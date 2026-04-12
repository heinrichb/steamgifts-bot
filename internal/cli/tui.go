package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heinrichb/steamgifts-bot/internal/account"
)

// runTUI starts the orchestrator and a bubbletea status dashboard side-by-side.
// The dashboard is a thin viewer over orchestrator.Snapshot() — the runners
// remain the source of truth for state, the TUI just renders it.
func runTUI(ctx context.Context, orch *account.Orchestrator, once bool) error {
	tuiCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- orch.Run(tuiCtx, once) }()

	model := dashboardModel{orch: orch, started: time.Now()}
	prog := tea.NewProgram(model, tea.WithContext(tuiCtx))
	if _, err := prog.Run(); err != nil {
		cancel()
		<-errCh
		return err
	}
	cancel()
	return <-errCh
}

type dashboardModel struct {
	orch    *account.Orchestrator
	started time.Time
	width   int
	height  int
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m dashboardModel) Init() tea.Cmd { return tick() }

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		return m, tick()
	}
	return m, nil
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func (m dashboardModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("steamgifts-bot — live status"))
	b.WriteString("  ")
	b.WriteString(dimStyle.Render(fmt.Sprintf("uptime %s", time.Since(m.started).Round(time.Second))))
	b.WriteString("\n\n")
	b.WriteString(headerStyle.Render(fmt.Sprintf("%-16s %-16s %-8s %-12s %-22s %s",
		"ACCOUNT", "USERNAME", "POINTS", "ENTRIES", "NEXT RUN", "LAST")))
	b.WriteString("\n")

	for _, st := range m.orch.Snapshot() {
		next := "-"
		if !st.NextRun.IsZero() {
			next = humanizeUntil(st.NextRun)
		}
		last := dimStyle.Render("-")
		if st.LastError != "" {
			last = errStyle.Render("ERR: " + truncate(st.LastError, 60))
		} else if !st.LastRun.IsZero() {
			last = okStyle.Render(humanizeAgo(st.LastRun))
		}
		fmt.Fprintf(&b, "%-16s %-16s %-8d %-12s %-22s %s\n",
			truncate(st.Name, 15),
			truncate(orDash(st.Username), 15),
			st.Points,
			fmt.Sprintf("%d ok / %d", st.EntriesOK, st.EntriesAttempt),
			next,
			last,
		)
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("press q to quit"))
	return b.String()
}

func humanizeUntil(t time.Time) string {
	d := time.Until(t).Round(time.Second)
	if d <= 0 {
		return "now"
	}
	return "in " + d.String()
}

func humanizeAgo(t time.Time) string {
	d := time.Since(t).Round(time.Second)
	return d.String() + " ago"
}

// truncate returns s limited to n runes (not bytes), with an ellipsis if
// it had to cut. Rune-safe so multi-byte UTF-8 names render correctly in
// the dashboard.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
