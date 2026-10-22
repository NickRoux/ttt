package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"

	"github.com/NickRoux/ttt/internal/language"
	"github.com/NickRoux/ttt/internal/transliterate"
	"github.com/NickRoux/ttt/internal/tui"
)

var version = "dev"

func main() {
	interactive := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, interactive); err != nil {
		fmt.Fprintln(os.Stderr, "ttt:", err)
		os.Exit(1)
	}
}

func run(args []string, input io.Reader, output, diagnostics io.Writer, interactive bool) error {
	pluginDir, err := language.DefaultDirectory()
	if err != nil {
		return fmt.Errorf("find plugin directory: %w", err)
	}
	flags := flag.NewFlagSet("ttt", flag.ContinueOnError)
	flags.SetOutput(diagnostics)
	id := flags.String("language", "ru", "language plugin id")
	directory := flags.String("plugins", pluginDir, "directory containing additional language JSON plugins")
	list := flags.Bool("list-languages", false, "list installed language plugins")
	showVersion := flags.Bool("version", false, "print version and exit")
	flags.Usage = func() {
		fmt.Fprint(diagnostics, "Usage: ttt [flags] [text]\n\nType in the terminal, convert text, or pipe stdin.\n\n")
		flags.PrintDefaults()
		fmt.Fprintln(diagnostics, "\nEditor: Ctrl+C copy · Ctrl+D finish · Esc English · F2 language · Ctrl+Q quit")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *showVersion {
		_, err := fmt.Fprintln(output, "ttt", version)
		return err
	}
	languages, err := language.Load(*directory)
	if err != nil {
		return err
	}
	if *list {
		for _, lang := range languages {
			if _, err := fmt.Fprintf(output, "%s\t%s\n", lang.ID, lang.Name); err != nil {
				return err
			}
		}
		return nil
	}
	active := -1
	for i, lang := range languages {
		if lang.ID == *id {
			active = i
			break
		}
	}
	if active < 0 {
		return fmt.Errorf("unknown language %q; use --list-languages", *id)
	}
	if flags.NArg() > 0 {
		_, err := fmt.Fprintln(output, transliterate.Convert(languages[active], strings.Join(flags.Args(), " ")))
		return err
	}
	if !interactive {
		text, err := io.ReadAll(input)
		if err != nil {
			return err
		}
		_, err = io.WriteString(output, transliterate.Convert(languages[active], string(text)))
		return err
	}
	result, err := tea.NewProgram(tui.New(languages, active), tea.WithOutput(diagnostics)).Run()
	if err != nil {
		return err
	}
	if text, finished := result.(tui.Model).Output(); finished {
		_, err = fmt.Fprintln(output, text)
	}
	return err
}
