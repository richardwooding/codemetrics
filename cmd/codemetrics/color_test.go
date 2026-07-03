package main

import (
	"os"
	"strings"
	"testing"
)

func TestColorEnabled(t *testing.T) {
	// A regular file is never a character device (stands in for a redirected pipe/file).
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	if !colorEnabled(f, "always") {
		t.Error(`"always" should enable color`)
	}
	if colorEnabled(f, "never") {
		t.Error(`"never" should disable color`)
	}

	t.Run("auto to a non-tty is off", func(t *testing.T) {
		t.Setenv("NO_COLOR", "x")   // track for restore …
		_ = os.Unsetenv("NO_COLOR") // … then ensure it's unset for this case
		if colorEnabled(f, "auto") {
			t.Error("auto to a redirected file must be off")
		}
	})

	t.Run("NO_COLOR disables auto", func(t *testing.T) {
		t.Setenv("NO_COLOR", "") // present, even empty → disabled (no-color.org)
		if colorEnabled(f, "auto") {
			t.Error("NO_COLOR present must disable auto")
		}
		if !colorEnabled(f, "always") {
			t.Error(`"always" overrides NO_COLOR`)
		}
	})
}

func TestPalettePaint(t *testing.T) {
	on := palette{on: true}
	off := palette{on: false}
	if got := off.cognitive("42"); got != "42" {
		t.Errorf("disabled palette should be a no-op, got %q", got)
	}
	if got := on.cognitive("42"); !strings.HasPrefix(got, "\x1b[") || !strings.HasSuffix(got, ansiReset) {
		t.Errorf("enabled palette should wrap in ANSI, got %q", got)
	}
	if on.cognitive("") != "" {
		t.Error("empty string should not be wrapped")
	}
}

func TestPrintTableColorToggle(t *testing.T) {
	cog := 42
	rows := []row{{File: "a.go", Function: "F", Cyclomatic: 10, Cognitive: &cog, StartLine: 1, EndLine: 9}}

	var off, on strings.Builder
	printTable(&off, rows, palette{on: false})
	printTable(&on, rows, palette{on: true})

	if strings.Contains(off.String(), "\x1b[") {
		t.Error("plain table must contain no ANSI escapes")
	}
	if !strings.Contains(on.String(), "\x1b[") {
		t.Error("colored table should contain ANSI escapes")
	}
	// Content is present regardless of color.
	for _, want := range []string{"COGNITIVE", "LOCATION", "a.go:1"} {
		if !strings.Contains(off.String(), want) {
			t.Errorf("table missing %q", want)
		}
	}
}
