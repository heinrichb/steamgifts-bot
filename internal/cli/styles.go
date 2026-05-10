package cli

import "github.com/charmbracelet/lipgloss"

var (
	brandColors = []string{"#06b6d4", "#3b82f6", "#8b5cf6", "#d946ef"}
	brandMid    = "#3b82f6"
	brandTo     = "#d946ef"

	styleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
	styleDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleOK      = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	styleOKBold  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82"))
	styleErr     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	styleErrBold = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	styleWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	styleBanner = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 2)

	styleFooter = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	styleFooterKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color(brandMid)).
			Bold(true)

	styleFooterDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	styleMenuItem = lipgloss.NewStyle().
			Padding(0, 2)

	styleMenuSelected = lipgloss.NewStyle().
				Foreground(lipgloss.Color(brandMid)).
				Bold(true).
				Padding(0, 2)

	styleMenuCursor = lipgloss.NewStyle().
			Foreground(lipgloss.Color(brandTo)).
			Bold(true)

	styleMenuDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 2)

	dotActive  = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("●")
	dotStopped = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("●")
	dotNone    = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○")

	logTime    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	logDebug   = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true)
	logInfo    = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	logWarn    = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	logError   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	logMsg     = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	logAccount = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	logKey     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	logVal     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	tableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("245")).
			Padding(0, 1)

	tableCell = lipgloss.NewStyle().
			Padding(0, 1)

	tableBorder = lipgloss.NewStyle().
			Foreground(lipgloss.Color("236"))

	styleProgressEmpty = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	styleProgressText  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	styleThumb = lipgloss.NewStyle().Foreground(lipgloss.Color(brandMid))
)

func statusOK(msg string) string {
	return "  " + styleOK.Render("✓") + " " + msg
}

func statusErr(msg string) string {
	return "  " + styleErr.Render("✗") + " " + msg
}

func footerHint(key, desc string) string {
	return styleFooterKey.Render(key) + " " + styleFooterDesc.Render(desc)
}
