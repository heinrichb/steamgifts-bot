package cli

import tea "github.com/charmbracelet/bubbletea"

type appPage interface {
	Init() tea.Cmd
	Update(tea.Msg) (appPage, tea.Cmd)
	View() string
	Breadcrumb() string
	FooterKeys() string
	FooterStatus() string
	Done() bool
}

type resultPage struct {
	breadcrumb string
	message    string
	back       bool
}

func newResultPage(breadcrumb, message string) resultPage {
	return resultPage{breadcrumb: breadcrumb, message: message}
}

func (p resultPage) Init() tea.Cmd { return nil }

func (p resultPage) Update(msg tea.Msg) (appPage, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "enter", "esc", "q", " ":
			p.back = true
		}
	}
	return p, nil
}

func (p resultPage) View() string {
	return "\n" + p.message + "\n"
}

func (p resultPage) Breadcrumb() string { return p.breadcrumb }

func (p resultPage) FooterKeys() string {
	return footerHint("enter/esc", "back")
}

func (p resultPage) FooterStatus() string { return "" }

func (p resultPage) Done() bool { return p.back }
