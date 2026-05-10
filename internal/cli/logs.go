package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func logFilePath(configPath string) string {
	if configPath != "" {
		return filepath.Join(filepath.Dir(configPath), "steamgifts-bot.log")
	}
	if p := findConfig(); p != "" {
		return filepath.Join(filepath.Dir(p), "steamgifts-bot.log")
	}
	return ""
}

func formatLogLine(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var fields map[string]any
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return raw
	}

	var b strings.Builder

	if t, ok := fields["time"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
			b.WriteString(logTime.Render(parsed.Format("15:04:05")))
		} else {
			b.WriteString(logTime.Render(t))
		}
		b.WriteString(" ")
	}

	if level, ok := fields["level"].(string); ok {
		padded := fmt.Sprintf("%-5s", level)
		switch strings.ToUpper(level) {
		case "DEBUG":
			b.WriteString(logDebug.Render(padded))
		case "INFO":
			b.WriteString(logInfo.Render(padded))
		case "WARN":
			b.WriteString(logWarn.Render(padded))
		case "ERROR":
			b.WriteString(logError.Render(padded))
		default:
			b.WriteString(padded)
		}
		b.WriteString(" ")
	}

	if acct, ok := fields["account"].(string); ok {
		b.WriteString(logAccount.Render("[" + acct + "]"))
		b.WriteString(" ")
	}

	if msg, ok := fields["msg"].(string); ok {
		b.WriteString(logMsg.Render(msg))
	}

	skip := map[string]bool{"time": true, "level": true, "msg": true, "account": true}
	var extras []string
	var keys []string
	for k := range fields {
		if !skip[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := fmt.Sprintf("%v", fields[k])
		extras = append(extras, logKey.Render(k)+"="+logVal.Render(v))
	}
	if len(extras) > 0 {
		b.WriteString("  ")
		b.WriteString(strings.Join(extras, " "))
	}

	return b.String()
}

const maxLogLines = 500

type logModel struct {
	path         string
	lines        []string
	file         *os.File
	reader       *bufio.Reader
	width        int
	height       int
	scroll       int
	hscroll      int
	maxLineWidth int
	err          error
	back         bool
}

type logTickMsg struct{}

func logTick() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return logTickMsg{} })
}

func newLogModel(path string) logModel {
	m := logModel{path: path}
	m.loadInitial()
	return m
}

func (m *logModel) loadInitial() {
	f, err := os.Open(m.path)
	if err != nil {
		m.err = err
		return
	}
	m.file = f

	info, err := f.Stat()
	if err != nil {
		m.err = err
		return
	}

	const tailBytes = 32768
	if info.Size() > tailBytes {
		if _, err := f.Seek(-tailBytes, io.SeekEnd); err == nil {
			r := bufio.NewReader(f)
			_, _ = r.ReadBytes('\n')
			m.reader = r
		}
	} else {
		m.reader = bufio.NewReader(f)
	}

	m.readAvailable()
	m.scrollToBottom()
}

func (m *logModel) readAvailable() {
	if m.reader == nil {
		return
	}
	for {
		line, err := m.reader.ReadString('\n')
		if line = strings.TrimRight(line, "\n\r"); line != "" {
			formatted := formatLogLine(line)
			if formatted != "" {
				m.lines = append(m.lines, formatted)
				if w := lipgloss.Width(formatted); w > m.maxLineWidth {
					m.maxLineWidth = w
				}
			}
		}
		if err != nil {
			break
		}
	}
	if len(m.lines) > maxLogLines {
		m.lines = m.lines[len(m.lines)-maxLogLines:]
	}
}

func (m *logModel) scrollToBottom() {
	visible := m.viewHeight()
	if len(m.lines) > visible {
		m.scroll = len(m.lines) - visible
	} else {
		m.scroll = 0
	}
}

func (m logModel) viewHeight() int {
	if m.height < 3 {
		return 20
	}
	if m.maxLineWidth > m.width {
		return m.height - 1
	}
	return m.height
}

func (m logModel) Init() tea.Cmd { return logTick() }

func (m logModel) Update(msg tea.Msg) (appPage, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			if m.file != nil {
				m.file.Close()
				m.file = nil
			}
			m.back = true
			return m, nil
		case "up", "w":
			if m.scroll > 0 {
				m.scroll--
			}
		case "down", "s":
			mx := len(m.lines) - m.viewHeight()
			if mx < 0 {
				mx = 0
			}
			if m.scroll < mx {
				m.scroll++
			}
		case "left", "a":
			m.hscroll -= 8
			if m.hscroll < 0 {
				m.hscroll = 0
			}
		case "right", "d":
			maxH := m.maxLineWidth - m.width
			if maxH > 0 && m.hscroll < maxH {
				m.hscroll += 8
				if m.hscroll > maxH {
					m.hscroll = maxH
				}
			}
		case "pgup":
			m.scroll -= m.viewHeight()
			if m.scroll < 0 {
				m.scroll = 0
			}
		case "pgdown":
			m.scroll += m.viewHeight()
			mx := len(m.lines) - m.viewHeight()
			if mx < 0 {
				mx = 0
			}
			if m.scroll > mx {
				m.scroll = mx
			}
		case "home", "g":
			m.scroll = 0
			m.hscroll = 0
		case "end", "G":
			m.scrollToBottom()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height - headerHeight() - footerHeight()
	case logTickMsg:
		if m.reader != nil {
			m.reader.Reset(m.file)
		}
		prevLen := len(m.lines)
		atBottom := m.scroll >= len(m.lines)-m.viewHeight()-1
		m.readAvailable()
		if atBottom && len(m.lines) > prevLen {
			m.scrollToBottom()
		}
		return m, logTick()
	}
	return m, nil
}

func (m logModel) View() string {
	if m.err != nil {
		return "\n  " + styleErr.Render(m.err.Error()) + "\n"
	}

	if len(m.lines) == 0 {
		return "\n  " + styleDim.Render("no log entries found") + "\n"
	}

	var b strings.Builder

	visible := m.viewHeight()
	start := m.scroll
	end := start + visible
	if end > len(m.lines) {
		end = len(m.lines)
	}

	for i := start; i < end; i++ {
		line := m.lines[i]
		if m.hscroll > 0 {
			line = hslice(line, m.hscroll, m.width)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if hbar := m.hscrollBar(); hbar != "" {
		b.WriteString(hbar)
	}

	return b.String()
}

func (m logModel) hscrollBar() string {
	if m.maxLineWidth <= m.width || m.width < 10 {
		return ""
	}

	barWidth := m.width - 2
	viewRatio := float64(m.width) / float64(m.maxLineWidth)
	thumbSize := max(int(float64(barWidth)*viewRatio), 1)

	maxHScroll := m.maxLineWidth - m.width
	thumbPos := 0
	if maxHScroll > 0 && m.hscroll > 0 {
		thumbPos = int(float64(m.hscroll) / float64(maxHScroll) * float64(barWidth-thumbSize))
	}

	var bar strings.Builder
	bar.WriteString(styleDim.Render("◀"))
	for i := 0; i < barWidth; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			bar.WriteString(styleThumb.Render("━"))
		} else {
			bar.WriteString(styleProgressEmpty.Render("─"))
		}
	}
	bar.WriteString(styleDim.Render("▶"))
	return bar.String()
}

// hslice returns a substring of an ANSI-styled string starting at visible
// character position start, up to width visible characters. ANSI escape
// sequences before the start position are preserved so styling is correct.
func hslice(s string, start, width int) string {
	if start <= 0 && width <= 0 {
		return s
	}

	runes := []rune(s)
	var ansiState strings.Builder
	var result strings.Builder
	visible := 0
	captured := 0
	started := false
	i := 0

	for i < len(runes) {
		if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			j := i + 2
			for j < len(runes) {
				if (runes[j] >= 'A' && runes[j] <= 'Z') || (runes[j] >= 'a' && runes[j] <= 'z') {
					j++
					break
				}
				j++
			}
			seq := string(runes[i:j])
			i = j

			if !started {
				ansiState.WriteString(seq)
			} else {
				result.WriteString(seq)
			}
			continue
		}

		visible++
		if visible > start {
			if width > 0 && captured >= width {
				break
			}
			if !started {
				started = true
				result.WriteString(ansiState.String())
			}
			result.WriteRune(runes[i])
			captured++
		}
		i++
	}

	return result.String()
}

func (m logModel) FooterStatus() string {
	var parts []string
	visible := m.viewHeight()
	if len(m.lines) > visible {
		mx := len(m.lines) - visible
		pct := 0
		if mx > 0 {
			pct = (m.scroll * 100) / mx
		}
		parts = append(parts, fmt.Sprintf("%d/%d (%d%%)", m.scroll+visible, len(m.lines), pct))
	}
	if m.hscroll > 0 {
		parts = append(parts, fmt.Sprintf("col %d", m.hscroll+1))
	}
	if len(parts) == 0 {
		return ""
	}
	return styleDim.Render(strings.Join(parts, "  "))
}

func (m logModel) Breadcrumb() string { return "service logs" }

func (m logModel) FooterKeys() string {
	keys := []string{
		footerHint("esc/q", "back"),
		footerHint("↑↓/ws", "scroll"),
	}
	if m.maxLineWidth > m.width {
		keys = append(keys, footerHint("←→/ad", "pan"))
	}
	keys = append(keys,
		footerHint("g/G", "top/bottom"),
		footerHint("PgUp/PgDn", "page"),
	)
	return strings.Join(keys, "    ")
}

func (m logModel) Done() bool { return m.back }
