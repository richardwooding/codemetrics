package main

import "os"

// palette adds ANSI SGR color to strings, or is a no-op when color is off.
// Colors mirror the marketing site's terminal: purple complexity numbers, dim
// header and locations, amber gate summary.
type palette struct{ on bool }

// newPalette decides whether writes to f should be colorized, given the
// --color mode.
func newPalette(f *os.File, mode string) palette {
	return palette{on: colorEnabled(f, mode)}
}

// colorEnabled reports whether to emit ANSI color for writes to f:
//
//   - "never"  → off.
//   - "always" → on (an explicit override — even when piped or NO_COLOR is set).
//   - "auto"   → on only when NO_COLOR is unset AND f is a terminal, so color is
//     dropped when output is redirected to a file or pipe.
//
// NO_COLOR follows https://no-color.org: any value (even empty) disables color.
func colorEnabled(f *os.File, mode string) bool {
	switch mode {
	case "never":
		return false
	case "always":
		return true
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

const ansiReset = "\x1b[0m"

func (p palette) paint(code, s string) string {
	if !p.on || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + ansiReset
}

func (p palette) header(s string) string    { return p.paint("2", s) }          // dim
func (p palette) cognitive(s string) string { return p.paint("1;38;5;141", s) } // bold purple
func (p palette) location(s string) string  { return p.paint("90", s) }         // gray
func (p palette) muted(s string) string     { return p.paint("2", s) }          // dim
func (p palette) warn(s string) string      { return p.paint("33", s) }         // amber
