package transliterate

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/NickRoux/ttt/internal/language"
)

func russian(t *testing.T) *language.Language {
	t.Helper()
	languages, err := language.Load("")
	if err != nil {
		t.Fatal(err)
	}
	return languages[0]
}

func TestRussianMessages(t *testing.T) {
	lang := russian(t)
	for _, test := range []struct{ input, want string }{
		{"Privet, mir!", "Привет, мир!"},
		{"Ja zhivu v Kanade. Kak dela?", "Я живу в Канаде. Как дела?"},
		{"Jo yo ö Zh CH Sh SHH w je ä ju yu ü ja ya q", "Ё ё ё Ж Ч Ш Щ щ э э ю ю ю я я я"},
		{"s+hodit' v raj+on", "сходить в район"},
		{"j+o j+e", "йо йе"},
		{"# ## ' '' tvz mjz TVZ MJZ", "ъ Ъ ь Ь ъ ь Ъ Ь"},
		{"Dobryj den'!\nUvidimsja v 18:30 😊", "Добрый день!\nУвидимся в 18:30 😊"},
		{"Уже русский — 123!?", "Уже русский — 123!?"},
		{"", ""},
	} {
		t.Run(test.input, func(t *testing.T) {
			if got := Convert(lang, test.input); got != test.want {
				t.Errorf("Convert(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestCompositionAndReset(t *testing.T) {
	engine := New(russian(t))
	for _, step := range []struct {
		key    rune
		erase  int
		insert string
	}{{'s', 0, "с"}, {'h', 1, "ш"}, {'h', 1, "щ"}, {'!', 0, "!"}} {
		erase, insert := engine.Push(step.key)
		if erase != step.erase || insert != step.insert {
			t.Errorf("Push(%q) = (%d, %q), want (%d, %q)", step.key, erase, insert, step.erase, step.insert)
		}
	}
	engine.Push('s')
	engine.Reset()
	if erase, insert := engine.Push('h'); erase != 0 || insert != "х" {
		t.Fatalf("composition crossed an edit boundary: (%d, %q)", erase, insert)
	}
}

func TestArbitraryPluginOverlappingKeysAndMultipleOutputRunes(t *testing.T) {
	lang, err := language.New(language.Pack{Version: 1, ID: "test", Name: "Test", Mappings: []language.Mapping{
		{Output: "α", Keys: []string{"a"}},
		{Output: "ω", Keys: []string{"abcd"}},
		{Output: "β", Keys: []string{"bc"}},
		{Output: "ks", Keys: []string{"x"}},
		{Output: "ψ", Keys: []string{"xy"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for input, want := range map[string]string{"abce": "αβe", "abcde": "ωe", "x": "ks", "xy": "ψ", "XY": "Ψ", "axb": "αksb"} {
		if got := Convert(lang, input); got != want {
			t.Errorf("Convert(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPendingStateStaysBounded(t *testing.T) {
	lang := russian(t)
	engine := New(lang)
	for _, r := range strings.Repeat("shhtvzmjzjoshchxyz", 1000) {
		engine.Push(r)
		if utf8.RuneCountInString(engine.pending) >= lang.MaxKey {
			t.Fatalf("pending state grew past the longest key: %q", engine.pending)
		}
	}
}

func BenchmarkConvert(b *testing.B) {
	languages, err := language.Load("")
	if err != nil {
		b.Fatal(err)
	}
	text := strings.Repeat("Privet! Ja zhivu v Kanade. ", 100)
	b.ReportAllocs()
	for b.Loop() {
		Convert(languages[0], text)
	}
}
