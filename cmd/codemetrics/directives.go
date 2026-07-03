package main

import (
	"regexp"
	"strings"
)

// Suppression directives let a source comment opt a function out of the
// complexity gate — like gocyclo's //gocyclo:ignore and golangci-lint's
// //nolint. A directive applies to the function whose declaration is on the
// directive's own line (trailing) or on the line directly below a contiguous
// block of comment lines carrying it (matching those tools' "immediately
// preceding" rule).
//
// Recognised, and the metric ids each suppresses:
//
//	codemetrics:ignore                 -> both        (all languages)
//	codemetrics:ignore cognitive       -> cognitive   (all languages)
//	codemetrics:ignore cyclomatic,...  -> named       (all languages)
//	gocyclo:ignore                     -> both        (Go only)
//	nolint / nolint:all                -> both        (Go only)
//	nolint:gocyclo                     -> cyclomatic  (Go only)
//	nolint:gocognit                    -> cognitive   (Go only)
//
// A nolint list naming neither gocyclo nor gocognit (e.g. //nolint:errcheck)
// suppresses nothing.

// Directives are anchored to the start of the comment body (after optional
// whitespace), matching golangci-lint's machine-readable convention — so prose
// that merely mentions "nolint" doesn't trigger suppression.
var (
	// codemetrics:ignore  or  codemetrics:ignore <metric>[,<metric>...]
	reCodemetrics = regexp.MustCompile(`^\s*codemetrics:ignore(?:\s+([a-z,\s]+))?`)
	// gocyclo:ignore
	reGocyclo = regexp.MustCompile(`^\s*gocyclo:ignore`)
	// nolint  or  nolint:<linter>[,<linter>...]  (golangci-lint)
	reNolint = regexp.MustCompile(`^\s*nolint(?::([\w,\s-]+))?`)
)

// lineCommentPrefixes maps a language id to its line-comment markers. Only the
// text after such a marker on a candidate line is scanned, so a directive-like
// token in code or a string literal is not matched. Unknown ids fall back to
// the common set.
var lineCommentPrefixes = map[string][]string{
	"go":         {"//"},
	"c":          {"//"},
	"cpp":        {"//"},
	"csharp":     {"//"},
	"java":       {"//"},
	"javascript": {"//"},
	"typescript": {"//"},
	"rust":       {"//"},
	"kotlin":     {"//"},
	"scala":      {"//"},
	"swift":      {"//"},
	"php":        {"//", "#"},
	"python":     {"#"},
	"ruby":       {"#"},
	"perl":       {"#"},
	"r":          {"#"},
	"matlab":     {"%"},
}

func commentPrefixesFor(lang string) []string {
	if p, ok := lineCommentPrefixes[lang]; ok {
		return p
	}
	return []string{"//", "#"}
}

// commentBody returns the comment text of a line — everything after the first
// line-comment marker for lang — and whether the line is (or contains) a
// comment at all. A line whose first non-space characters are a comment marker
// is a full comment line; a marker later in the line is a trailing comment.
func commentBody(line, lang string) (body string, isComment bool) {
	best := -1
	for _, p := range commentPrefixesFor(lang) {
		if i := strings.Index(line, p); i >= 0 && (best < 0 || i < best) {
			best = i + len(p)
		}
	}
	if best < 0 {
		return "", false
	}
	return line[best:], true
}

// isFullComment reports whether a line's first non-space run is a comment marker.
func isFullComment(line, lang string) bool {
	t := strings.TrimSpace(line)
	if t == "" {
		return false
	}
	for _, p := range commentPrefixesFor(lang) {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// ignoredMetrics returns the sorted rule ids ("cognitive","cyclomatic") that a
// directive suppresses for the function declared at startLine (1-based) in
// lines. It inspects the function's own line (trailing directive) and the
// contiguous run of comment lines immediately above it.
func ignoredMetrics(lines []string, lang string, startLine int) []string {
	if startLine < 1 || startLine > len(lines) {
		return nil
	}
	var cognitive, cyclomatic bool
	apply := func(cog, cyc bool) {
		cognitive = cognitive || cog
		cyclomatic = cyclomatic || cyc
	}

	// Trailing directive on the declaration line, then contiguous comments above.
	scan := func(line string) {
		body, ok := commentBody(line, lang)
		if !ok {
			return
		}
		cog, cyc := parseDirective(body, lang)
		apply(cog, cyc)
	}

	scan(lines[startLine-1]) // declaration line (trailing comment)
	for i := startLine - 2; i >= 0; i-- {
		if !isFullComment(lines[i], lang) {
			break // stops at first blank/non-comment line — enforces contiguity
		}
		scan(lines[i])
	}

	switch {
	case cognitive && cyclomatic:
		return []string{ruleCognitive, ruleCyclomatic}
	case cognitive:
		return []string{ruleCognitive}
	case cyclomatic:
		return []string{ruleCyclomatic}
	default:
		return nil
	}
}

// parseDirective interprets a single comment body, returning whether it
// suppresses cognitive and/or cyclomatic complexity.
func parseDirective(body, lang string) (cognitive, cyclomatic bool) {
	if m := reCodemetrics.FindStringSubmatch(body); m != nil {
		metrics := strings.TrimSpace(m[1])
		if metrics == "" {
			return true, true // unqualified: both
		}
		for _, w := range strings.FieldsFunc(metrics, func(r rune) bool { return r == ',' || r == ' ' }) {
			switch w {
			case "cognitive":
				cognitive = true
			case "cyclomatic":
				cyclomatic = true
			}
		}
		return cognitive, cyclomatic
	}

	// gocyclo/nolint are Go-ecosystem directives; only honor them for Go.
	if lang != "go" {
		return false, false
	}
	if reGocyclo.MatchString(body) {
		return true, true
	}
	if m := reNolint.FindStringSubmatch(body); m != nil {
		linters := strings.TrimSpace(m[1])
		if linters == "" || linters == "all" {
			return true, true // bare //nolint or //nolint:all
		}
		for _, w := range strings.FieldsFunc(linters, func(r rune) bool { return r == ',' || r == ' ' }) {
			switch w {
			case "gocyclo":
				cyclomatic = true
			case "gocognit":
				cognitive = true
			}
		}
		return cognitive, cyclomatic
	}
	return false, false
}
