package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLI(t *testing.T) {
	for _, test := range []struct {
		name, input, want string
		args              []string
	}{
		{name: "version", args: []string{"--version"}, want: "ttt dev\n"},
		{name: "arguments", args: []string{"Privet, mir!"}, want: "Привет, мир!\n"},
		{name: "pipe preserves newline", input: "Dobryj den'!\n", want: "Добрый день!\n"},
		{name: "empty pipe", want: ""},
		{name: "Greek plugin", args: []string{"--plugins", "examples", "--language", "el", "Geia"}, want: "Γεια\n"},
		{name: "list", args: []string{"--plugins", "examples", "--list-languages"}, want: "ru\tRussian\nel\tGreek\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"--plugins", t.TempDir()}, test.args...)
			var output, diagnostics bytes.Buffer
			if err := run(args, strings.NewReader(test.input), &output, &diagnostics, false); err != nil {
				t.Fatal(err)
			}
			if output.String() != test.want || diagnostics.Len() != 0 {
				t.Fatalf("stdout = %q, diagnostics = %q; want %q", output.String(), diagnostics.String(), test.want)
			}
		})
	}
}

func TestUnknownLanguageAndHelp(t *testing.T) {
	var output, diagnostics bytes.Buffer
	if err := run([]string{"--plugins", t.TempDir(), "--language", "missing"}, strings.NewReader(""), &output, &diagnostics, false); err == nil {
		t.Fatal("unknown language accepted")
	}
	if err := run([]string{"--help"}, strings.NewReader(""), &output, &diagnostics, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diagnostics.String(), "Usage: ttt [flags] [text]") {
		t.Fatal("help missing the ttt command name")
	}
}
