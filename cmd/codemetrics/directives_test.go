package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestIgnoredMetrics(t *testing.T) {
	cases := []struct {
		name string
		lang string
		src  string // the function decl is the LAST line; directives precede/trail it
		want []string
	}{
		{
			name: "gocyclo:ignore above -> both",
			lang: "go",
			src:  "//gocyclo:ignore\nfunc f() {",
			want: []string{ruleCognitive, ruleCyclomatic},
		},
		{
			name: "nolint:gocyclo above -> cyclomatic only",
			lang: "go",
			src:  "//nolint:gocyclo\nfunc f() {",
			want: []string{ruleCyclomatic},
		},
		{
			name: "nolint:gocyclo trailing -> cyclomatic only",
			lang: "go",
			src:  "func f() { //nolint:gocyclo",
			want: []string{ruleCyclomatic},
		},
		{
			name: "nolint:gocognit -> cognitive only",
			lang: "go",
			src:  "//nolint:gocognit\nfunc f() {",
			want: []string{ruleCognitive},
		},
		{
			name: "nolint:gocyclo,gocognit -> both",
			lang: "go",
			src:  "//nolint:gocyclo,gocognit\nfunc f() {",
			want: []string{ruleCognitive, ruleCyclomatic},
		},
		{
			name: "bare nolint -> both",
			lang: "go",
			src:  "//nolint\nfunc f() {",
			want: []string{ruleCognitive, ruleCyclomatic},
		},
		{
			name: "nolint naming other linters -> nothing",
			lang: "go",
			src:  "//nolint:errcheck,gosec\nfunc f() {",
			want: nil,
		},
		{
			name: "codemetrics:ignore cognitive -> cognitive",
			lang: "go",
			src:  "// codemetrics:ignore cognitive\nfunc f() {",
			want: []string{ruleCognitive},
		},
		{
			name: "codemetrics:ignore unqualified -> both",
			lang: "go",
			src:  "// codemetrics:ignore\nfunc f() {",
			want: []string{ruleCognitive, ruleCyclomatic},
		},
		{
			name: "blank line breaks association -> nothing",
			lang: "go",
			src:  "//gocyclo:ignore\n\nfunc f() {",
			want: nil,
		},
		{
			name: "directive-looking token in code, not a comment -> nothing",
			lang: "go",
			src:  "x := \"gocyclo:ignore\"\nfunc f() {",
			want: nil,
		},
		{
			name: "contiguous doc block, directive on earlier line -> both",
			lang: "go",
			src:  "//gocyclo:ignore\n// f does a thing.\nfunc f() {",
			want: []string{ruleCognitive, ruleCyclomatic},
		},
		{
			name: "gocyclo/nolint ignored for non-Go languages",
			lang: "javascript",
			src:  "//nolint:gocyclo\nfunction f() {",
			want: nil,
		},
		{
			name: "python codemetrics:ignore with # comment -> both",
			lang: "python",
			src:  "# codemetrics:ignore\ndef f():",
			want: []string{ruleCognitive, ruleCyclomatic},
		},
		{
			name: "matlab codemetrics:ignore with % comment -> both",
			lang: "matlab",
			src:  "% codemetrics:ignore\nfunction f()",
			want: []string{ruleCognitive, ruleCyclomatic},
		},
		{
			name: "no directive -> nothing",
			lang: "go",
			src:  "// ordinary comment\nfunc f() {",
			want: nil,
		},
	}
	for _, tc := range cases {
		lines := strings.Split(tc.src, "\n")
		startLine := len(lines) // the decl is the last line
		got := ignoredMetrics(lines, tc.lang, startLine)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: ignoredMetrics = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestComputeFindings_RespectsIgnore(t *testing.T) {
	cog := 99
	rows := []row{
		{File: "a.go", Function: "Loud", Cyclomatic: 50, Cognitive: &cog, StartLine: 1, EndLine: 9},
		{File: "a.go", Function: "Quiet", Cyclomatic: 50, Cognitive: &cog, StartLine: 20, EndLine: 29,
			Ignored: []string{ruleCognitive, ruleCyclomatic}},
		{File: "a.go", Function: "HalfQuiet", Cyclomatic: 50, Cognitive: &cog, StartLine: 40, EndLine: 49,
			Ignored: []string{ruleCyclomatic}},
	}
	got := computeFindings(rows, 10, 10)

	byFn := map[string][]string{}
	for _, f := range got {
		byFn[f.Function] = append(byFn[f.Function], f.Rule)
	}
	if len(byFn["Loud"]) != 2 {
		t.Errorf("Loud should have both findings, got %v", byFn["Loud"])
	}
	if _, ok := byFn["Quiet"]; ok {
		t.Errorf("Quiet is fully suppressed, should have no findings, got %v", byFn["Quiet"])
	}
	if rules := byFn["HalfQuiet"]; len(rules) != 1 || rules[0] != ruleCognitive {
		t.Errorf("HalfQuiet should keep only the cognitive finding, got %v", rules)
	}
}
