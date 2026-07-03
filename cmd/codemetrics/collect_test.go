package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectRoutesByExtension(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.go", `package p
func f(n int) int {
	if n > 0 {
		return 1
	}
	return 0
}`)
	writeFile(t, dir, "calc.py", `def classify(n):
    if n < 0:
        return "neg"
    return "pos"
`)
	writeFile(t, dir, "notes.txt", "not source code, must be ignored")

	rows, err := collect([]string{dir}, "")
	if err != nil {
		t.Fatal(err)
	}
	byFn := map[string]row{}
	for _, r := range rows {
		byFn[r.Function] = r
	}
	if _, ok := byFn["f"]; !ok {
		t.Error("Go function f not analyzed (go/ast backend)")
	}
	if _, ok := byFn["classify"]; !ok {
		t.Error("Python function classify not analyzed (tree-sitter backend)")
	}
	if len(rows) != 2 {
		t.Errorf("want exactly 2 rows (.txt ignored), got %d: %+v", len(rows), rows)
	}
	// Go always reports cognitive; both metrics should be present and > 0.
	if r := byFn["f"]; r.Cognitive == nil || r.Cyclomatic < 1 {
		t.Errorf("Go metrics look wrong: %+v", r)
	}
}

func TestCollectForceLang(t *testing.T) {
	dir := t.TempDir()
	// A .txt file that is really Go; --lang forces the Go backend.
	writeFile(t, dir, "hidden.txt", `package p
func g() { if true { _ = 1 } }`)

	rows, err := collect([]string{filepath.Join(dir, "hidden.txt")}, "go")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Function != "g" {
		t.Fatalf("forced go lang did not analyze .txt as Go: %+v", rows)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
