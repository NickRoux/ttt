package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/NickRoux/ttt/internal/language"
)

func editorModel(t *testing.T) Model {
	t.Helper()
	languages, err := language.Load("../../examples")
	if err != nil {
		t.Fatal(err)
	}
	return New(languages, 0)
}

func update(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func typeText(m Model, text string) Model {
	for _, r := range text {
		m = update(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

func control(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl} }

func TestLiveTypingEditingAndUndo(t *testing.T) {
	m := typeText(editorModel(t), "shh")
	if m.editor.Value() != "щ" {
		t.Fatalf("live composition = %q", m.editor.Value())
	}
	m = update(m, control('z'))
	if m.editor.Value() != "ш" {
		t.Fatalf("undo = %q", m.editor.Value())
	}
	m = update(m, control('y'))
	if m.editor.Value() != "щ" {
		t.Fatalf("redo = %q", m.editor.Value())
	}
	m = update(m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = typeText(m, "s")
	m = update(m, tea.KeyPressMsg{Code: tea.KeyLeft})
	m = typeText(m, "h")
	if m.editor.Value() != "хс" {
		t.Fatalf("composition crossed cursor movement: %q", m.editor.Value())
	}
}

func TestEnglishModeAndPaste(t *testing.T) {
	m := typeText(editorModel(t), "Privet! ")
	m = update(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m = typeText(m, "https://example.com + ")
	m = update(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m = update(m, tea.PasteMsg{Content: "Kak dela?\nУже русский 😊"})
	if want := "Привет! https://example.com + Как дела?\nУже русский 😊"; m.editor.Value() != want {
		t.Fatalf("mixed typing and paste = %q, want %q", m.editor.Value(), want)
	}
	m = update(m, control('z'))
	if strings.Contains(m.editor.Value(), "Как") {
		t.Fatal("paste was not one undo operation")
	}
}

func TestSwitchLanguagePreservesExistingText(t *testing.T) {
	m := typeText(editorModel(t), "Privet ")
	m = update(m, tea.KeyPressMsg{Code: tea.KeyF2})
	m = update(m, tea.KeyPressMsg{Code: tea.KeyDown})
	m = update(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = typeText(m, "Geia")
	if m.languages[m.active].ID != "el" || m.editor.Value() != "Привет Γεια" {
		t.Fatalf("language switch: %s, %q", m.languages[m.active].ID, m.editor.Value())
	}
}

func TestFinishAndQuit(t *testing.T) {
	m := typeText(editorModel(t), "Privet")
	quit := update(m, control('q'))
	if _, finished := quit.Output(); finished {
		t.Fatal("quit unexpectedly emitted output")
	}
	m = update(m, control('d'))
	if text, finished := m.Output(); !finished || text != "Привет" {
		t.Fatalf("finish = (%q, %v)", text, finished)
	}
}

func TestClipboardFeedback(t *testing.T) {
	m := editorModel(t)
	m = update(m, clipboardResult{text: "Привет"})
	if !strings.Contains(m.status, "Copied") {
		t.Fatal("successful copy feedback missing")
	}
	m = update(m, clipboardResult{text: "Привет", err: errors.New("unavailable")})
	if !strings.Contains(m.status, "OSC 52") {
		t.Fatal("clipboard fallback feedback missing")
	}
	m = update(m, pasteResult{err: errors.New("unavailable")})
	if !strings.Contains(m.status, "unavailable") {
		t.Fatal("clipboard error missing")
	}
}

func TestCopyShortcuts(t *testing.T) {
	for _, shortcut := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"Ctrl+C", control('c')},
		{"Cmd+C", tea.KeyPressMsg{Code: 'c', Mod: tea.ModSuper}},
		{"Meta+C", tea.KeyPressMsg{Code: 'c', Mod: tea.ModMeta}},
		{"Ctrl+Shift+C", tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl | tea.ModShift}},
		{"Ctrl+S", control('s')},
	} {
		for _, selected := range []bool{false, true} {
			t.Run(shortcut.name+map[bool]string{false: "/whole-message", true: "/selection"}[selected], func(t *testing.T) {
				m := typeText(editorModel(t), "Privet, mir!")
				if selected {
					for range 4 {
						m = update(m, tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift})
					}
				}
				want := "Привет, мир!"
				if selected && shortcut.name != "Ctrl+S" {
					want = "мир!"
				}
				var copied string
				m.writeClipboard = func(text string) error { copied = text; return nil }
				next, cmd := m.Update(shortcut.key)
				if cmd == nil {
					t.Fatal("copy shortcut returned no command")
				}
				result, ok := cmd().(clipboardResult)
				if !ok {
					t.Fatal("copy shortcut did not request a clipboard write")
				}
				if copied != want || result.text != want || result.err != nil {
					t.Fatalf("copied %q with result %+v; want %q", copied, result, want)
				}
				m = update(next.(Model), result)
				if m.editor.Value() != "Привет, мир!" || m.finished || !strings.Contains(m.status, "Copied") {
					t.Fatal("copy changed the document, finished the editor, or omitted feedback")
				}
			})
		}
	}
}

func TestCopyFromOverlays(t *testing.T) {
	for _, overlay := range []rune{tea.KeyF1, tea.KeyF2} {
		m := typeText(editorModel(t), "Privet")
		m = update(m, tea.KeyPressMsg{Code: overlay})
		var copied string
		m.writeClipboard = func(text string) error { copied = text; return nil }
		_, cmd := m.Update(control('c'))
		if cmd == nil {
			t.Fatal("copy unavailable in overlay")
		}
		if _, ok := cmd().(clipboardResult); !ok || copied != "Привет" {
			t.Fatalf("overlay copy = %q", copied)
		}
	}
}

func TestLayoutFitsTerminal(t *testing.T) {
	for _, size := range []struct{ width, height int }{{48, 20}, {60, 22}, {80, 24}, {100, 32}, {160, 48}} {
		m := update(editorModel(t), tea.WindowSizeMsg{Width: size.width, Height: size.height})
		for _, overlay := range []rune{0, tea.KeyF1, tea.KeyF2} {
			viewModel := m
			if overlay != 0 {
				viewModel = update(m, tea.KeyPressMsg{Code: overlay})
			}
			view := viewModel.View().Content
			if width, height := lipgloss.Width(view), lipgloss.Height(view); width > size.width || height > size.height {
				t.Errorf("%dx%d overlay %d rendered %dx%d", size.width, size.height, overlay, width, height)
			}
		}
	}
}
