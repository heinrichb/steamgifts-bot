package cli

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// confirmField is an embeddable two-option confirm prompt.
type confirmField struct {
	title       string
	description string
	options     [2]string
	cursor      int
	done        bool
	value       bool
	cancelled   bool
}

func (f *confirmField) reset(title, desc, affirm, deny string) {
	*f = confirmField{
		title:       title,
		description: desc,
		options:     [2]string{affirm, deny},
	}
}

func (f *confirmField) Update(msg tea.Msg) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "w", "left", "a":
			f.cursor = 0
		case "down", "s", "right", "d":
			f.cursor = 1
		case "enter":
			f.done = true
			f.value = f.cursor == 0
		case "esc", "q":
			f.cancelled = true
		}
	}
}

func (f confirmField) View() string {
	var b strings.Builder
	b.WriteString("\n")
	if f.title != "" {
		b.WriteString("  " + styleMenuSelected.Render(f.title) + "\n")
	}
	if f.description != "" {
		for _, line := range strings.Split(f.description, "\n") {
			b.WriteString("  " + styleDim.Render(line) + "\n")
		}
	}
	b.WriteString("\n")
	for i, opt := range f.options {
		cursor := "  "
		label := styleMenuItem.Render(opt)
		if i == f.cursor {
			cursor = styleMenuCursor.Render("▸ ")
			label = styleMenuSelected.Render(opt)
		}
		b.WriteString("  " + cursor + label + "\n")
	}
	return b.String()
}

// inputField is an embeddable text input prompt.
type inputField struct {
	title       string
	description string
	ti          textinput.Model
	validate    func(string) error
	errMsg      string
	done        bool
	cancelled   bool
}

func (f *inputField) reset(title, desc, value string, password bool, validate func(string) error) {
	ti := textinput.New()
	ti.SetValue(value)
	ti.Focus()
	ti.PromptStyle = styleMenuCursor
	ti.TextStyle = styleFooterDesc
	ti.Cursor.Style = styleMenuCursor
	if password {
		ti.EchoMode = textinput.EchoPassword
	}
	*f = inputField{
		title:       title,
		description: desc,
		ti:          ti,
		validate:    validate,
	}
}

func (f *inputField) Init() tea.Cmd {
	return textinput.Blink
}

func (f *inputField) Update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "enter":
			val := f.ti.Value()
			if f.validate != nil {
				if err := f.validate(val); err != nil {
					f.errMsg = err.Error()
					return nil
				}
			}
			f.done = true
			return nil
		case "esc":
			f.cancelled = true
			return nil
		}
	}
	f.errMsg = ""
	var cmd tea.Cmd
	f.ti, cmd = f.ti.Update(msg)
	return cmd
}

func (f inputField) View() string {
	var b strings.Builder
	b.WriteString("\n")
	if f.title != "" {
		b.WriteString("  " + styleMenuSelected.Render(f.title) + "\n")
	}
	if f.description != "" {
		b.WriteString("  " + styleDim.Render(f.description) + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + f.ti.View() + "\n")
	if f.errMsg != "" {
		b.WriteString("  " + styleErr.Render(f.errMsg) + "\n")
	}
	return b.String()
}

// noteField is an embeddable informational display.
type noteField struct {
	title       string
	description string
	done        bool
	cancelled   bool
}

func (f *noteField) reset(title, desc string) {
	*f = noteField{title: title, description: desc}
}

func (f *noteField) Update(msg tea.Msg) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "enter", " ":
			f.done = true
		case "esc", "q":
			f.cancelled = true
		}
	}
}

func (f noteField) View() string {
	var b strings.Builder
	b.WriteString("\n")
	if f.title != "" {
		b.WriteString("  " + styleMenuSelected.Render(f.title) + "\n")
	}
	b.WriteString("\n")
	if f.description != "" {
		for _, line := range strings.Split(f.description, "\n") {
			b.WriteString("  " + styleDim.Render(line) + "\n")
		}
	}
	return b.String()
}
