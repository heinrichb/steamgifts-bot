package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	wcBrand  = lipgloss.Color("#3b82f6")
	wcPink   = lipgloss.Color("#d946ef")
	wcCyan   = lipgloss.Color("#06b6d4")
	wcGreen  = lipgloss.Color("82")
	wcRed    = lipgloss.Color("196")
	wcDim    = lipgloss.Color("240")
	wcNormal = lipgloss.Color("252")
	wcMuted  = lipgloss.Color("245")

	wcTitle       = lipgloss.NewStyle().Bold(true).Foreground(wcBrand)
	wcDesc        = lipgloss.NewStyle().Foreground(wcMuted)
	wcCursorStyle = lipgloss.NewStyle().Foreground(wcPink).Bold(true)
	wcSelected    = lipgloss.NewStyle().Foreground(wcBrand).Bold(true)
	wcUnselected  = lipgloss.NewStyle().Foreground(wcNormal)
	wcErrStyle    = lipgloss.NewStyle().Foreground(wcRed)
	wcDimStyle    = lipgloss.NewStyle().Foreground(wcDim)
	wcFooterKey   = lipgloss.NewStyle().Foreground(wcBrand).Bold(true)
	wcFooterDesc  = lipgloss.NewStyle().Foreground(wcMuted)
)

// --- Confirm ---

type confirmModel struct {
	title       string
	description string
	affirm      string
	deny        string
	cursor      int
	chosen      bool
	value       bool
	cancelled   bool
}

func newConfirm(title, description, affirm, deny string) confirmModel {
	return confirmModel{
		title:       title,
		description: description,
		affirm:      affirm,
		deny:        deny,
	}
}

func (m confirmModel) Init() tea.Cmd { return nil }

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k", "left", "h":
			m.cursor = 0
		case "down", "j", "right", "l":
			m.cursor = 1
		case "enter":
			m.chosen = true
			m.value = m.cursor == 0
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + wcTitle.Render(m.title) + "\n")
	if m.description != "" {
		b.WriteString("  " + wcDesc.Render(m.description) + "\n")
	}
	b.WriteString("\n")

	options := []string{m.affirm, m.deny}
	for i, opt := range options {
		cursor := "  "
		label := wcUnselected.Render(opt)
		if i == m.cursor {
			cursor = wcCursorStyle.Render("▸ ")
			label = wcSelected.Render(opt)
		}
		b.WriteString("  " + cursor + label + "\n")
	}

	b.WriteString("\n")
	b.WriteString("  " + wcFooterKey.Render("↑↓") + " " + wcFooterDesc.Render("choose") + "    " +
		wcFooterKey.Render("enter") + " " + wcFooterDesc.Render("confirm") + "    " +
		wcFooterKey.Render("esc") + " " + wcFooterDesc.Render("cancel") + "\n")
	return b.String()
}

func runConfirm(title, description, affirm, deny string) (bool, error) {
	m := newConfirm(title, description, affirm, deny)
	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return false, err
	}
	cm := result.(confirmModel)
	if cm.cancelled {
		return false, fmt.Errorf("cancelled")
	}
	return cm.value, nil
}

// --- Text Input ---

type inputModel struct {
	title       string
	description string
	input       textinput.Model
	validate    func(string) error
	err         string
	done        bool
	cancelled   bool
}

func newInput(title, description, value string, echoPassword bool, validate func(string) error) inputModel {
	ti := textinput.New()
	ti.SetValue(value)
	ti.Focus()
	ti.PromptStyle = lipgloss.NewStyle().Foreground(wcPink)
	ti.TextStyle = lipgloss.NewStyle().Foreground(wcNormal)
	ti.CursorStyle = lipgloss.NewStyle().Foreground(wcCyan)
	if echoPassword {
		ti.EchoMode = textinput.EchoPassword
	}

	return inputModel{
		title:       title,
		description: description,
		input:       ti,
		validate:    validate,
	}
}

func (m inputModel) Init() tea.Cmd { return textinput.Blink }

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			val := m.input.Value()
			if m.validate != nil {
				if err := m.validate(val); err != nil {
					m.err = err.Error()
					return m, nil
				}
			}
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	m.err = ""
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m inputModel) View() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + wcTitle.Render(m.title) + "\n")
	if m.description != "" {
		b.WriteString("  " + wcDesc.Render(m.description) + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + m.input.View() + "\n")
	if m.err != "" {
		b.WriteString("  " + wcErrStyle.Render(m.err) + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + wcFooterKey.Render("enter") + " " + wcFooterDesc.Render("submit") + "    " +
		wcFooterKey.Render("esc") + " " + wcFooterDesc.Render("cancel") + "\n")
	return b.String()
}

func runInput(title, description, value string, echoPassword bool, validate func(string) error) (string, error) {
	m := newInput(title, description, value, echoPassword, validate)
	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", err
	}
	im := result.(inputModel)
	if im.cancelled {
		return "", fmt.Errorf("cancelled")
	}
	return im.input.Value(), nil
}

// --- Note ---

type noteModel struct {
	title       string
	description string
	done        bool
	cancelled   bool
}

func newNote(title, description string) noteModel {
	return noteModel{title: title, description: description}
}

func (m noteModel) Init() tea.Cmd { return nil }

func (m noteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter", " ":
			m.done = true
			return m, tea.Quit
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m noteModel) View() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + wcTitle.Render(m.title) + "\n\n")
	for _, line := range strings.Split(m.description, "\n") {
		b.WriteString("  " + wcDesc.Render(line) + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + wcFooterKey.Render("enter") + " " + wcFooterDesc.Render("continue") + "    " +
		wcFooterKey.Render("esc") + " " + wcFooterDesc.Render("cancel") + "\n")
	return b.String()
}

func runNote(title, description string) error {
	m := newNote(title, description)
	result, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	nm := result.(noteModel)
	if nm.cancelled {
		return fmt.Errorf("cancelled")
	}
	return nil
}
