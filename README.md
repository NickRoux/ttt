# ttt

A small terminal editor for writing Russian with a Latin keyboard. Type `Privet, mir!` and see `Привет, мир!` as you type, with a key guide above your message. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), Bubbles, and Lip Gloss.

Everything runs locally. Language plugins are JSON files, so new alphabets can be added without rebuilding the binary.

## Run

Requires Go 1.26 or newer.

Install and run:

```sh
go install github.com/NickRoux/ttt@latest
ttt
```

Ensure `GOBIN` (or `~/go/bin` by default) is on your PATH.

From a checkout:

```sh
go run .
```

Or build a standalone binary:

```sh
go build -o bin/ttt .
./bin/ttt
```

To install the checkout as the `ttt` command, run `go install .`.

## Editor

| Key | Action |
| --- | --- |
| Latin letters | Insert transliterated characters immediately |
| Enter | New line |
| Arrows, Home/End, Ctrl+Home/End | Move through the message |
| Shift+arrows, Ctrl+G | Select text / select all |
| Backspace, Delete, Ctrl+W | Delete a character / previous word |
| Ctrl+C / Cmd+C (when forwarded by the terminal) | Copy selected text, or the complete message if nothing is selected |
| Ctrl+S | Copy the complete message |
| Ctrl+V / terminal paste | Paste, converting according to the current mode |
| Esc | Toggle English passthrough for URLs, names, or literal punctuation |
| F1 | Show all keys and aliases; arrows/Page Up/Page Down scroll |
| F2 | Choose an installed language; existing text stays as typed |
| Ctrl+Z / Ctrl+Y | Undo / redo (up to 200 edits) |
| Ctrl+D | Finish and print the message to stdout |
| Ctrl+Q | Quit |

The interface adapts to dark/light terminals and terminal size (minimum 48 × 20). Short terminals prioritize combinations; taller terminals show the alphabet. F1 always provides the full guide.

Copy uses the system clipboard, with OSC 52 as a fallback for compatible terminals and SSH sessions. If clipboard reading is unavailable, use your terminal's paste shortcut. Pasting while in English mode preserves Latin text.

Ctrl+C copies without exiting the editor. Cmd+C is also recognized when your terminal forwards Command-key events to the application (reported as Super/Meta+C). Some terminals reserve Cmd+C for copying their own terminal selection; in that case use Ctrl+C, or configure your terminal to send Ctrl+C for Cmd+C. Terminal shortcuts are handled before application input; see the [Ghostty keybinding reference](https://ghostty.org/docs/config/keybind/reference) and [Kitty keyboard protocol](https://sw.kovidgoyal.net/kitty/keyboard-protocol/).

The UI goes to stderr, so you can save a completed message with `ttt > message.txt`, then Ctrl+D. Drafts are kept in memory for the current session.

## Russian keys

The built-in pack follows the [default translit.net mappings](https://translit.net/), including:

| Input | Output | Input | Output |
| --- | --- | --- | --- |
| `zh` | ж | `ch` | ч |
| `sh` | ш | `shh`, `w` | щ |
| `jo`, `yo`, `ö` | ё | `je`, `ä` | э |
| `ju`, `yu`, `ü` | ю | `ja`, `ya`, `q` | я |
| `j` | й | `y` | ы |
| `'`, `mjz` | ь | `#`, `tvz` | ъ |
| `''` | Ь | `##` | Ъ |

Uppercase the first input letter for uppercase output: `Zh → Ж`, `SHH → Щ`.

Use `+` as an invisible separator: `s+hodit' → сходить`, `raj+on → район`. Switch to English mode to insert a literal `+`, apostrophe, or `#`. Combinations compose only while typing continuously; moving the cursor, editing, or switching modes commits the visible characters.

## Commands and pipes

```sh
ttt 'Privet, mir!'
printf "Dobryj den'!\n" | ttt
ttt --list-languages
ttt --plugins ./examples --language el 'Geia'
```

All flags precede text; use `--` before text starting with a hyphen. Piped input preserves trailing newlines. Argument input prints a trailing newline.

## Language plugins

Russian is embedded in the binary as `internal/language/packs/ru.json`. External packs use the exact same format. Copy a JSON file into the plugin directory and restart the utility, or point `--plugins` at a directory. No compilation or executable plugin code is needed.

Default plugin directories follow `os.UserConfigDir`:

- macOS: `~/Library/Application Support/ttt/languages/`
- Linux: `$XDG_CONFIG_HOME/ttt/languages/`, or `~/.config/ttt/languages/`
- Windows: `%AppData%\ttt\languages\`

A minimal pack:

```json
{
  "version": 1,
  "id": "custom",
  "name": "My alphabet",
  "separator": "+",
  "mappings": [
    { "output": "α", "keys": ["a"] },
    { "output": "θ", "keys": ["th"] }
  ]
}
```

The `id` selects the language in `--language`; `name` appears in the UI. Mapping order controls the guide. The first key is shown in the compact guide; F1 lists all aliases. `separator` can be omitted or empty to disable it, or one character to break combinations without inserting text.

Keys are case-insensitive and may contain 1–16 printable, non-space Unicode characters. The longest matching key wins. Capitalizing the first character uppercases the whole output, which may contain multiple characters. Unmapped input passes through. The engine does letter substitution; it does not infer spelling, grammar, or context-dependent forms.

Plugins must use version 1, have a unique lowercase ID containing letters, digits, or hyphens, and define nonempty names, outputs, and keys. Duplicate aliases, control characters, unknown JSON fields, trailing JSON, and files larger than 1 MiB are rejected with the filename. Plugins cannot replace the built-in Russian ID. Non-JSON files are ignored.

`examples/el.json` is a working Greek example with explicit accent keys (`a' → ά`) and final sigma (`s' → ς`):

```sh
go run . --plugins ./examples --language el
```

Architecture:

- `internal/language`: versioned pack format, validation, discovery, compiled lookups.
- `internal/transliterate`: shared incremental engine for live typing and batch conversion. Retains only the possible prefix of a longer combination, bounded by the longest key.
- `internal/tui`: editor, guide, language picker, clipboard, and undo history. No Russian-specific rules.
- `main.go`: flags, pipes, and terminal startup.

## Verify

```sh
go test -race ./...
go vet ./...
go test ./internal/transliterate -bench . -benchmem
```
