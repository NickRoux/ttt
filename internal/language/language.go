// Package language loads and validates portable, data-only language plugins.
package language

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

//go:embed packs/*.json
var builtins embed.FS

// Mapping lists the equivalent input sequences for an output character or string.
type Mapping struct {
	Output string   `json:"output"`
	Keys   []string `json:"keys"`
}

// Pack is the versioned on-disk plugin format. Order determines the key guide.
type Pack struct {
	Version   int       `json:"version"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Separator string    `json:"separator"`
	Mappings  []Mapping `json:"mappings"`
}

// Language is an immutable, validated language pack with compiled lookup tables.
type Language struct {
	Pack
	lookup   map[string]string
	prefixes map[string]bool
	MaxKey   int
}

// New compiles a pack. Keys are case-insensitive; uppercase input capitalizes output.
func New(pack Pack) (*Language, error) {
	if pack.Version != 1 {
		return nil, fmt.Errorf("unsupported plugin version %d (want 1)", pack.Version)
	}
	if pack.ID == "" || strings.TrimSpace(pack.Name) == "" || len(pack.Mappings) == 0 {
		return nil, fmt.Errorf("id, name, and mappings are required")
	}
	for _, r := range pack.ID {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
			return nil, fmt.Errorf("id %q must contain only lowercase ASCII letters, digits, or hyphens", pack.ID)
		}
	}
	if !printable(pack.Name) || !printable(pack.Separator) || utf8.RuneCountInString(pack.Separator) > 1 {
		return nil, fmt.Errorf("name must be printable; separator must be empty or one printable character")
	}
	lang := &Language{Pack: pack, lookup: make(map[string]string), prefixes: make(map[string]bool)}
	for _, mapping := range pack.Mappings {
		if mapping.Output == "" || !printable(mapping.Output) || len(mapping.Keys) == 0 {
			return nil, fmt.Errorf("each mapping needs printable output and at least one key")
		}
		for _, key := range mapping.Keys {
			lower := strings.ToLower(key)
			runes := []rune(lower)
			if len(runes) == 0 || len(runes) > 16 || !printable(key) || strings.ContainsFunc(key, unicode.IsSpace) {
				return nil, fmt.Errorf("key %q must have 1–16 printable, non-space characters", key)
			}
			if pack.Separator != "" && strings.Contains(lower, strings.ToLower(pack.Separator)) {
				return nil, fmt.Errorf("key %q contains the separator", key)
			}
			if _, exists := lang.lookup[lower]; exists {
				return nil, fmt.Errorf("duplicate key %q", key)
			}
			lang.lookup[lower] = mapping.Output
			lang.MaxKey = max(lang.MaxKey, len(runes))
			for i := 1; i < len(runes); i++ {
				lang.prefixes[string(runes[:i])] = true
			}
		}
	}
	return lang, nil
}

func printable(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsFunc(s, func(r rune) bool { return !unicode.IsPrint(r) })
}

func (l *Language) Lookup(key string) (string, bool) {
	value, ok := l.lookup[strings.ToLower(key)]
	return value, ok
}

func (l *Language) Prefix(key string) bool {
	return l.prefixes[strings.ToLower(key)]
}

// DefaultDirectory follows the platform's user configuration directory.
func DefaultDirectory() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ttt", "languages"), nil
}

// Load returns built-ins followed by plugins sorted by filename. A missing plugin
// directory is fine; malformed packs and duplicate IDs are actionable errors.
func Load(directory string) ([]*Language, error) {
	langs, err := loadFS(builtins, "packs")
	if err != nil {
		return nil, err
	}
	if directory == "" {
		return langs, nil
	}
	info, err := os.Stat(directory)
	if os.IsNotExist(err) {
		return langs, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open plugin directory %s: %w", directory, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("plugin path %s is not a directory", directory)
	}
	plugins, err := loadFS(os.DirFS(directory), ".")
	if err != nil {
		return nil, fmt.Errorf("load plugins from %s: %w", directory, err)
	}
	langs = append(langs, plugins...)
	ids := make(map[string]bool)
	for _, lang := range langs {
		if ids[lang.ID] {
			return nil, fmt.Errorf("duplicate language id %q", lang.ID)
		}
		ids[lang.ID] = true
	}
	return langs, nil
}

func loadFS(source fs.FS, dir string) ([]*Language, error) {
	entries, err := fs.ReadDir(source, dir)
	if err != nil {
		return nil, err
	}
	var langs []*Language
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		file, err := source.Open(path.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		lang, decodeErr := decode(file)
		closeErr := file.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), decodeErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		langs = append(langs, lang)
	}
	return langs, nil
}

func decode(reader io.Reader) (*Language, error) {
	const maxSize = 1 << 20
	data, err := io.ReadAll(io.LimitReader(reader, maxSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSize {
		return nil, fmt.Errorf("plugin exceeds 1 MiB")
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var pack Pack
	if err := decoder.Decode(&pack); err != nil {
		return nil, err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("plugin must contain exactly one JSON object")
	}
	return New(pack)
}
