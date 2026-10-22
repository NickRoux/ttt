// Package transliterate implements a language-independent, incremental engine.
package transliterate

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NickRoux/ttt/internal/language"
)

// Engine retains only the uncommitted suffix that could grow into a longer key.
// Its size is bounded by the longest key in the active language plugin.
type Engine struct {
	language *language.Language
	pending  string
	rendered string
}

func New(lang *language.Language) *Engine { return &Engine{language: lang} }

// Reset commits visible output and prevents composition across edits or modes.
func (e *Engine) Reset() { e.pending, e.rendered = "", "" }

// Push returns how many preceding output runes to replace and their replacement.
// For example: s inserts с, then h replaces с with ш, then h replaces ш with щ.
func (e *Engine) Push(r rune) (erase int, insert string) {
	if string(r) == e.language.Separator {
		e.Reset()
		return 0, ""
	}
	erase = utf8.RuneCountInString(e.rendered)
	source := []rune(e.pending + string(r))
	e.Reset()
	var output strings.Builder
	for i := 0; i < len(source); {
		remaining := string(source[i:])
		if e.language.Prefix(remaining) {
			e.pending = remaining
			e.rendered = e.render(source[i:])
			output.WriteString(e.rendered)
			break
		}
		text, consumed := e.match(source[i:])
		output.WriteString(text)
		i += consumed
	}
	return erase, output.String()
}

func (e *Engine) render(source []rune) string {
	var output strings.Builder
	for len(source) > 0 {
		text, consumed := e.match(source)
		output.WriteString(text)
		source = source[consumed:]
	}
	return output.String()
}

func (e *Engine) match(source []rune) (string, int) {
	for size := min(len(source), e.language.MaxKey); size > 0; size-- {
		if output, ok := e.language.Lookup(string(source[:size])); ok {
			if unicode.IsUpper(source[0]) {
				output = strings.ToUpper(output)
			}
			return output, size
		}
	}
	return string(source[0]), 1
}

// Convert uses the same engine as live typing, including overlapping mappings.
func Convert(lang *language.Language, input string) string {
	engine := New(lang)
	output := make([]rune, 0, utf8.RuneCountInString(input))
	for _, r := range input {
		erase, insert := engine.Push(r)
		output = append(output[:len(output)-erase], []rune(insert)...)
	}
	return string(output)
}
