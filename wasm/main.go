//go:build js && wasm

// Command wasm exposes codemetrics to the browser for the playground page:
//
//	cmAnalyze(language string, source string) -> JSON string
//
// Go is analysed via the root package (go/ast); every other language via the
// treesitter subpackage. The playground build embeds a grammar subset (see the
// pages workflow) to keep the binary small — the CLI supports all languages.
package main

import (
	"encoding/json"
	"sort"
	"syscall/js"

	codemetrics "github.com/richardwooding/codemetrics"
	"github.com/richardwooding/codemetrics/treesitter"
)

type functionJSON struct {
	Name       string `json:"name"`
	Cyclomatic int    `json:"cyclomatic"`
	Cognitive  *int   `json:"cognitive"`
	StartLine  int    `json:"startLine"`
	EndLine    int    `json:"endLine"`
	Lines      int    `json:"lines"`
}

type resultJSON struct {
	Language  string         `json:"language"`
	Functions []functionJSON `json:"functions"`
	Error     string         `json:"error,omitempty"`
}

func analyze(language, source string) resultJSON {
	src := []byte(source)
	var (
		metrics []codemetrics.FunctionMetrics
		err     error
	)
	if language == "go" {
		metrics, err = codemetrics.ParseGo(src)
	} else {
		metrics, err = treesitter.Parse(language, src)
	}
	out := resultJSON{Language: language, Functions: make([]functionJSON, 0, len(metrics))}
	if err != nil {
		out.Error = err.Error()
		return out
	}
	for _, m := range metrics {
		out.Functions = append(out.Functions, functionJSON{
			Name:       m.QualifiedName(),
			Cyclomatic: m.Cyclomatic,
			Cognitive:  m.Cognitive,
			StartLine:  m.StartLine,
			EndLine:    m.EndLine,
			Lines:      m.Lines(),
		})
	}
	sort.SliceStable(out.Functions, func(i, j int) bool {
		return out.Functions[i].StartLine < out.Functions[j].StartLine
	})
	return out
}

func main() {
	js.Global().Set("cmAnalyze", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 2 {
			b, _ := json.Marshal(resultJSON{Error: "cmAnalyze requires (language, source)"})
			return string(b)
		}
		res := analyze(args[0].String(), args[1].String())
		b, err := json.Marshal(res)
		if err != nil {
			eb, _ := json.Marshal(resultJSON{Error: "internal: " + err.Error()})
			return string(eb)
		}
		return string(b)
	}))

	select {} // keep the Go runtime alive for future calls
}
