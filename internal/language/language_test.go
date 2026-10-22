package language

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePlugin(t *testing.T, directory, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadExternalPlugin(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, "custom.json", `{"version":1,"id":"custom","name":"Custom","separator":"+","mappings":[{"output":"λ","keys":["l","ll"]}]}`)
	writePlugin(t, dir, "README.txt", "ignored")
	languages, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(languages) != 2 || languages[0].ID != "ru" || languages[1].ID != "custom" {
		t.Fatalf("unexpected plugins: %v", languages)
	}
	if output, ok := languages[1].Lookup("LL"); !ok || output != "λ" {
		t.Fatalf("external lookup = (%q, %v)", output, ok)
	}
	if !languages[1].Prefix("L") {
		t.Fatal("external plugin prefixes were not compiled")
	}
}

func TestMissingPluginDirectory(t *testing.T) {
	languages, err := Load(filepath.Join(t.TempDir(), "missing"))
	if err != nil || len(languages) != 1 {
		t.Fatalf("missing directory should retain Russian: %v, %v", languages, err)
	}
}

func TestPluginValidation(t *testing.T) {
	valid := Pack{Version: 1, ID: "test", Name: "Test", Separator: "+", Mappings: []Mapping{{Output: "λ", Keys: []string{"l"}}}}
	for _, test := range []struct {
		name   string
		mutate func(*Pack)
	}{
		{"version", func(p *Pack) { p.Version = 2 }},
		{"id", func(p *Pack) { p.ID = "../escape" }},
		{"empty name", func(p *Pack) { p.Name = "" }},
		{"terminal escapes", func(p *Pack) { p.Name = "\x1b[31m" }},
		{"separator", func(p *Pack) { p.Separator = "++" }},
		{"no mappings", func(p *Pack) { p.Mappings = nil }},
		{"empty output", func(p *Pack) { p.Mappings = []Mapping{{Keys: []string{"l"}}} }},
		{"output escape", func(p *Pack) { p.Mappings = []Mapping{{Output: "\n", Keys: []string{"l"}}} }},
		{"empty keys", func(p *Pack) { p.Mappings = []Mapping{{Output: "λ"}} }},
		{"duplicate", func(p *Pack) { p.Mappings = []Mapping{{Output: "λ", Keys: []string{"l", "L"}}} }},
		{"separator in key", func(p *Pack) { p.Mappings = []Mapping{{Output: "λ", Keys: []string{"l+"}}} }},
		{"whitespace key", func(p *Pack) { p.Mappings = []Mapping{{Output: "λ", Keys: []string{"l l"}}} }},
		{"unbounded key", func(p *Pack) { p.Mappings = []Mapping{{Output: "λ", Keys: []string{strings.Repeat("l", 17)}}} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			pack := valid
			test.mutate(&pack)
			if _, err := New(pack); err == nil {
				t.Fatal("invalid plugin accepted")
			}
		})
	}
}

func TestMalformedPluginsFailWithFilename(t *testing.T) {
	for _, content := range []string{
		`{"version":`,
		`{"version":1,"unknown":true}`,
		`{"version":1} {}`,
		strings.Repeat(" ", (1<<20)+1),
	} {
		dir := t.TempDir()
		writePlugin(t, dir, "broken.json", content)
		if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "broken.json") {
			t.Fatalf("expected named plugin error, got %v", err)
		}
	}
}

func TestCannotShadowBuiltin(t *testing.T) {
	dir := t.TempDir()
	data, err := builtins.ReadFile("packs/ru.json")
	if err != nil {
		t.Fatal(err)
	}
	writePlugin(t, dir, "shadow.json", string(data))
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "duplicate language id") {
		t.Fatalf("expected duplicate language error, got %v", err)
	}
}

func TestPluginFileErrorsAreNotTreatedAsMissingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "missing.json"), filepath.Join(dir, "broken.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("broken plugin symlink silently discarded all plugins")
	}
	writePlugin(t, dir, "file", "not a directory")
	if _, err := Load(filepath.Join(dir, "file")); err == nil {
		t.Fatal("non-directory plugin path accepted")
	}
}

func TestExamplePlugin(t *testing.T) {
	languages, err := Load("../../examples")
	if err != nil {
		t.Fatal(err)
	}
	if len(languages) != 2 || languages[1].ID != "el" {
		data, _ := json.Marshal(languages)
		t.Fatalf("Greek example did not load: %s", data)
	}
}
