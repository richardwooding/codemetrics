package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	codemetrics "github.com/richardwooding/go-codemetrics"
	"github.com/richardwooding/go-codemetrics/treesitter"
	"github.com/richardwooding/projectdetect"
)

// row is one analyzed function.
type row struct {
	File       string `json:"file"`
	Function   string `json:"function"`
	Cyclomatic int    `json:"cyclomatic"`
	Cognitive  *int   `json:"cognitive,omitempty"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
}

// collect analyzes every source named by args (files, directories, or stdin
// when empty) and returns one row per function. forceLang, when non-empty,
// overrides per-file extension detection for every input.
func collect(args []string, forceLang string) ([]row, error) {
	if len(args) == 0 {
		src, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		lang := forceLang
		if lang == "" {
			lang = "go" // stdin defaults to Go, as before; override with --lang
		}
		return rowsFor("<stdin>", lang, src)
	}
	var out []row
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			r, err := collectDir(arg, forceLang)
			if err != nil {
				return nil, err
			}
			out = append(out, r...)
			continue
		}
		lang := langFor(arg, forceLang)
		if lang == "" {
			continue // unrecognised file type when named explicitly: skip quietly
		}
		r, err := rowsForFile(arg, lang)
		if err != nil {
			return nil, err
		}
		out = append(out, r...)
	}
	return out, nil
}

// collectDir walks root recursively. Build-artefact directories come from
// projectdetect (vendor, node_modules, target, __pycache__, …); dot-directories
// and testdata are skipped too. Only files with a recognised language are read.
func collectDir(root, forceLang string) ([]row, error) {
	excludes := map[string]struct{}{}
	if ex, err := projectdetect.CollectBuildExcludes(context.Background(), root); err == nil {
		for _, e := range ex {
			excludes[e] = struct{}{}
		}
	}
	var out []row
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "testdata" {
				return fs.SkipDir
			}
			if _, skip := excludes[name]; skip {
				return fs.SkipDir
			}
			return nil
		}
		lang := langFor(path, forceLang)
		if lang == "" {
			return nil
		}
		r, ferr := rowsForFile(path, lang)
		if ferr != nil {
			return ferr
		}
		out = append(out, r...)
		return nil
	})
	return out, err
}

// langFor resolves the language id for a path: the forced language if set,
// otherwise projectdetect's extension-based detection.
func langFor(path, forceLang string) string {
	if forceLang != "" {
		return forceLang
	}
	return projectdetect.LanguageForPath(path)
}

func rowsForFile(path, lang string) ([]row, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return rowsFor(path, lang, src)
}

func rowsFor(name, lang string, src []byte) ([]row, error) {
	fns, err := analyze(lang, src)
	if err != nil {
		// A language we have no analyzer for is skipped, not fatal — keeps
		// projectdetect's id set a safe superset of what we can analyze.
		if errors.Is(err, codemetrics.ErrUnsupportedLanguage) {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	out := make([]row, 0, len(fns))
	for _, f := range fns {
		out = append(out, row{
			File:       name,
			Function:   f.QualifiedName(),
			Cyclomatic: f.Cyclomatic,
			Cognitive:  f.Cognitive,
			StartLine:  f.StartLine,
			EndLine:    f.EndLine,
		})
	}
	return out, nil
}

// analyze dispatches to the Go (go/ast) or tree-sitter backend by language id.
func analyze(lang string, src []byte) ([]codemetrics.FunctionMetrics, error) {
	if lang == "go" {
		return codemetrics.ParseGo(src)
	}
	return treesitter.Parse(lang, src)
}
