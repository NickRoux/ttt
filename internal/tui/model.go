// Package tui provides the terminal editor. Language rules live in plugins.
package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/NickRoux/ttt/internal/language"
	"github.com/NickRoux/ttt/internal/transliterate"
)

type snapshot struct {
	text         string
	line, column int
}

type clipboardResult struct {
	text string
	err  error
}
type pasteResult struct {
	text string
	err  error
}
type clearStatus int

// Model owns editor state and the selected plugin, with no language-specific rules.
type Model struct {
	editor         textarea.Model
	languages      []*language.Language
	active         int
	engine         *transliterate.Engine
	latin          bool
	picker         bool
	selected       int
	allKeys        bool
	guideOffset    int
	width          int
	height         int
	dark           bool
	status         string
	statusID       int
	undo, redo     []snapshot
	finished       bool
	writeClipboard func(string) error
}

func New(languages []*language.Language, active int) Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.ShowLineNumbers = true
	editor.CharLimit = 0
	editor.MaxHeight = 0
	editor.MaxWidth = 0
	editor.Placeholder = "Type with Latin keys. Your text appears here…"
	editor.Focus()
	m := Model{
		editor: editor, languages: languages, active: active,
		dark: true, width: 80, height: 24, writeClipboard: clipboard.WriteAll,
	}
	m.engine = transliterate.New(languages[active])
	m.applyTheme()
	m.resize()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.RequestBackgroundColor)
}

// Output is emitted to stdout only when the user finishes with Ctrl+D.
func (m Model) Output() (string, bool) { return m.editor.Value(), m.finished }

func (m Model) current() snapshot {
	return snapshot{m.editor.Value(), m.editor.Line(), m.editor.Column()}
}

func (m *Model) remember(before snapshot) {
	if before.text == m.editor.Value() {
		return
	}
	m.undo = append(m.undo, before)
	if len(m.undo) > 200 {
		m.undo = m.undo[1:]
	}
	m.redo = nil
}

func (m *Model) restore(state snapshot) {
	m.engine.Reset()
	m.editor.SetValue(state.text)
	m.editor.MoveToBegin()
	for m.editor.Line() < state.line {
		m.editor.CursorDown()
	}
	m.editor.SetCursorColumn(state.column)
}

func (m *Model) notify(text string) tea.Cmd {
	m.status = text
	m.statusID++
	id := m.statusID
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearStatus(id) })
}

func (m *Model) updateEditor(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return cmd
}

func (m Model) copyText(text string) tea.Cmd {
	return func() tea.Msg { return clipboardResult{text, m.writeClipboard(text)} }
}

func (m *Model) insert(text string, paste bool) tea.Cmd {
	before := m.current()
	var commands []tea.Cmd
	if paste || m.editor.HasSelection() {
		m.engine.Reset()
	}
	if paste || m.latin {
		if !m.latin {
			text = transliterate.Convert(m.languages[m.active], text)
		}
		commands = append(commands, m.updateEditor(tea.PasteMsg{Content: text}))
		m.engine.Reset()
	} else {
		for _, r := range text {
			erase, insert := m.engine.Push(r)
			for range erase {
				commands = append(commands, m.updateEditor(tea.KeyPressMsg{Code: tea.KeyBackspace}))
			}
			if insert != "" {
				commands = append(commands, m.updateEditor(tea.PasteMsg{Content: insert}))
			}
		}
	}
	m.remember(before)
	return tea.Batch(commands...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
		m.applyTheme()
		return m, nil
	case clearStatus:
		if int(msg) == m.statusID {
			m.status = ""
		}
		return m, nil
	case clipboardResult:
		if msg.err != nil {
			cmd := tea.Batch(tea.SetClipboard(msg.text), m.notify("Sent to terminal clipboard (OSC 52)"))
			return m, cmd
		}
		cmd := m.notify("Copied to clipboard ✓")
		return m, cmd
	case pasteResult:
		if msg.err != nil {
			cmd := m.notify("Clipboard unavailable; use your terminal’s paste shortcut")
			return m, cmd
		}
		cmd := m.insert(msg.text, true)
		return m, cmd
	case tea.PasteMsg:
		if m.picker || m.allKeys {
			return m, nil
		}
		cmd := m.insert(msg.Content, true)
		return m, cmd
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+q":
			return m, tea.Quit
		case "ctrl+c", "super+c", "meta+c", "ctrl+shift+c":
			text := m.editor.Value()
			if m.editor.HasSelection() {
				text = m.editor.SelectedText()
			}
			return m, m.copyText(text)
		case "ctrl+s":
			return m, m.copyText(m.editor.Value())
		case "ctrl+d":
			m.finished = true
			return m, tea.Quit
		}
		if m.picker {
			return m.updatePicker(msg)
		}
		if m.allKeys {
			return m.updateGuide(msg)
		}
		switch msg.String() {
		case "ctrl+v":
			m.engine.Reset()
			return m, func() tea.Msg {
				text, err := clipboard.ReadAll()
				return pasteResult{text, err}
			}
		case "esc":
			m.latin = !m.latin
			m.engine.Reset()
			return m, nil
		case "f1":
			m.engine.Reset()
			m.allKeys, m.guideOffset = true, 0
			return m, nil
		case "f2":
			m.engine.Reset()
			m.picker, m.selected = true, m.active
			return m, nil
		case "ctrl+z":
			if len(m.undo) > 0 {
				m.redo = append(m.redo, m.current())
				last := len(m.undo) - 1
				m.restore(m.undo[last])
				m.undo = m.undo[:last]
			}
			return m, nil
		case "ctrl+y":
			if len(m.redo) > 0 {
				m.undo = append(m.undo, m.current())
				last := len(m.redo) - 1
				m.restore(m.redo[last])
				m.redo = m.redo[:last]
			}
			return m, nil
		}
		if msg.Text != "" && msg.Mod & ^tea.ModShift == 0 {
			cmd := m.insert(msg.Text, false)
			return m, cmd
		}
		m.engine.Reset()
		before := m.current()
		cmd := m.updateEditor(msg)
		m.remember(before)
		return m, cmd
	}
	cmd := m.updateEditor(msg)
	return m, cmd
}

func (m Model) updateGuide(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "f1", "enter":
		m.allKeys = false
	case "up", "k":
		m.guideOffset--
	case "down", "j":
		m.guideOffset++
	case "pgup":
		m.guideOffset -= m.height - 10
	case "pgdown":
		m.guideOffset += m.height - 10
	}
	m.guideOffset = max(0, min(m.guideOffset, len(m.aliasRows())-max(1, m.height-10)))
	return m, nil
}

func (m Model) updatePicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "f2":
		m.picker = false
	case "up", "k":
		m.selected = (m.selected + len(m.languages) - 1) % len(m.languages)
	case "down", "j":
		m.selected = (m.selected + 1) % len(m.languages)
	case "enter":
		m.active = m.selected
		m.engine = transliterate.New(m.languages[m.active])
		m.picker, m.latin = false, false
		m.resize()
	}
	return m, nil
}

func (m Model) counter() string {
	text := m.editor.Value()
	return fmt.Sprintf("%d chars · %d words", utf8.RuneCountInString(text), len(strings.Fields(text)))
}
