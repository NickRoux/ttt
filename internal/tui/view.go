package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type palette struct{ accent, text, muted, border string }

func (m Model) colors() palette {
	if m.dark {
		return palette{"#A5E4CC", "#E7E9F0", "#969EAF", "#475164"}
	}
	return palette{"#176C57", "#22283B", "#646C7E", "#A7B1C2"}
}

func ink(value, color string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(value)
}

func (m *Model) applyTheme() {
	p := m.colors()
	styles := textarea.DefaultStyles(m.dark)
	styles.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color(p.text))
	styles.Focused.CursorLine = lipgloss.NewStyle()
	styles.Focused.LineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color(p.border))
	styles.Focused.CursorLineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color(p.accent))
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color(p.muted))
	styles.Cursor.Color = lipgloss.Color(p.accent)
	m.editor.SetStyles(styles)
}

func (m Model) contentWidth() int { return max(20, min(116, m.width-4)) }

func (m Model) panel(content string, accent bool) string {
	border := m.colors().border
	if accent {
		border = m.colors().accent
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(border)).Padding(0, 1).
		Width(m.contentWidth() - 2).Render(content)
}

func (m Model) header() string {
	p := m.colors()
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.accent)).Render("т  ttt")
	mode := strings.ToUpper(m.languages[m.active].Name)
	if m.latin {
		mode = "ENGLISH · passthrough"
	}
	mode = ansi.Truncate(mode, max(1, m.contentWidth()-lipgloss.Width(title)-2), "…")
	gap := max(1, m.contentWidth()-lipgloss.Width(title)-lipgloss.Width(mode))
	return title + strings.Repeat(" ", gap) + ink(mode, p.accent)
}

func (m Model) guide() string {
	p := m.colors()
	inner := m.contentWidth() - 4
	cellWidth := 8
	columns := max(1, inner/cellWidth)
	var rows []string
	var row strings.Builder
	for i, mapping := range m.languages[m.active].Mappings {
		keys := mapping.Keys[0]
		cell := ink(mapping.Output, p.accent) + " " + ink(keys, p.text)
		cell = ansi.Truncate(cell, cellWidth-1, "…")
		row.WriteString(cell)
		row.WriteString(strings.Repeat(" ", max(0, cellWidth-lipgloss.Width(cell))))
		if (i+1)%columns == 0 || i == len(m.languages[m.active].Mappings)-1 {
			rows = append(rows, strings.TrimRight(row.String(), " "))
			row.Reset()
		}
	}
	if m.height < 25 {
		// Keep combinations visible in short terminals; F1 reveals every alias.
		rows = nil
		var combinations []string
		for _, mapping := range m.languages[m.active].Mappings {
			if len([]rune(mapping.Keys[0])) > 1 {
				combinations = append(combinations, mapping.Keys[0]+" "+mapping.Output)
			}
		}
		wrapped := ansi.Wrap(strings.Join(combinations, "   "), inner, " ")
		rows = strings.Split(wrapped, "\n")
	}
	rows = rows[:min(len(rows), max(1, (m.height-12)/2))]
	label := "KEY GUIDE  ·  F1 all keys & aliases"
	return m.panel(ink(label, p.muted)+"\n"+strings.Join(rows, "\n"), false)
}

func (m Model) aliasRows() []string {
	var rows []string
	for _, mapping := range m.languages[m.active].Mappings {
		line := mapping.Output + "  ←  " + strings.Join(mapping.Keys, ", ")
		rows = append(rows, strings.Split(ansi.Wrap(line, m.contentWidth()-4, " "), "\n")...)
	}
	return rows
}

func (m Model) allKeysView() string {
	rows := m.aliasRows()
	visible := max(1, m.height-10)
	start := max(0, min(m.guideOffset, len(rows)-visible))
	end := min(len(rows), start+visible)
	content := ink("ALL KEYS · "+m.languages[m.active].Name, m.colors().accent) + "\n\n" +
		strings.Join(rows[start:end], "\n") + "\n\n" +
		ink(fmt.Sprintf("↑/↓ scroll · F1/Esc back   %d–%d/%d", start+1, end, len(rows)), m.colors().muted)
	return m.header() + "\n\n" + m.panel(content, true)
}

func (m Model) footer() string {
	p := m.colors()
	status := m.status
	if status == "" {
		status = m.counter()
		if separator := m.languages[m.active].Separator; separator != "" {
			status += "   ·   " + separator + " separates combinations"
		}
	}
	help := "^C copy  ^D finish  Esc English  F2 language  ^Z undo  ^Q quit"
	if m.width < 68 {
		help = "^C copy · ^D finish · Esc mode · F2 lang · ^Q quit"
	}
	return ink(ansi.Truncate(status, m.contentWidth(), "…"), p.muted) + "\n" +
		ink(ansi.Truncate(help, m.contentWidth(), "…"), p.muted)
}

func (m Model) content() string {
	p := m.colors()
	editor := m.panel(ink("MESSAGE", p.muted)+"\n\n"+m.editor.View(), true)
	return m.header() + "\n\n" + m.guide() + "\n\n" + editor + "\n" + m.footer()
}

func (m *Model) resize() {
	m.editor.SetWidth(m.contentWidth() - 4)
	m.editor.SetHeight(1)
	// Measure the actual chrome so plugin sizes and terminal widths stay in sync.
	available := m.height - 2 - lipgloss.Height(m.content())
	m.editor.SetHeight(max(1, 1+available))
}

func (m Model) pickerView() string {
	p := m.colors()
	lines := []string{ink("LANGUAGES", p.accent), ""}
	visible := max(1, m.height-12)
	start := max(0, min(m.selected-visible/2, len(m.languages)-visible))
	for i := start; i < min(start+visible, len(m.languages)); i++ {
		lang := m.languages[i]
		line := fmt.Sprintf("  %s  ·  %s", lang.Name, lang.ID)
		if i == m.selected {
			line = ink("› "+lang.Name+"  ·  "+lang.ID, p.accent)
		}
		lines = append(lines, ansi.Truncate(line, m.contentWidth()-4, "…"))
	}
	lines = append(lines, "", ink("↑/↓ choose · Enter switch · Esc back", p.muted))
	return m.header() + "\n\n" + m.panel(strings.Join(lines, "\n"), true)
}

func (m Model) View() tea.View {
	content := m.content()
	if m.picker {
		content = m.pickerView()
	}
	if m.allKeys {
		content = m.allKeysView()
	}
	if m.width < 48 || m.height < 20 {
		content = "ttt\nResize to at least 48 × 20.\nCtrl+D finish · Ctrl+Q quit"
	}
	view := tea.NewView(lipgloss.NewStyle().Padding(1, 2).Render(content))
	view.AltScreen = true
	return view
}
