//go:build js && wasm

// Command wasm exposes codemetrics to the browser for the playground page:
//
//	cmAnalyze(language string, source string) -> JSON string
//	cmHighlight(language string, source string) -> JSON string
//
// Go is analysed via the root package (go/ast); every other language via the
// treesitter subpackage. The playground build embeds a grammar subset (see the
// pages workflow) to keep the binary small — the CLI supports all languages.
package main

import (
	"encoding/json"
	"sort"
	"strings"
	"syscall/js"
	"unicode/utf16"

	ts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
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

type highlightSpanJSON struct {
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Capture string `json:"capture"`
}

type highlightJSON struct {
	Language string              `json:"language"`
	Spans    []highlightSpanJSON `json:"spans"`
}

// highlightInherits supplies the upstream tree-sitter convention for grammars
// whose bundled highlight query holds only language-specific additions and
// whose registry entry does not (yet) record its parent.
var highlightInherits = map[string]string{
	"typescript": "javascript",
	"tsx":        "typescript",
	"cpp":        "c",
}

// emptyHighlightResult answers cmHighlight calls with bad arguments or a
// marshalling failure; marshalled once at startup.
var emptyHighlightResult = func() string {
	b, _ := json.Marshal(highlightJSON{Spans: []highlightSpanJSON{}})
	return string(b)
}()

// highlighters caches one Highlighter per playground language; nil marks a
// language whose grammar or highlight query is unavailable in this build, so
// its editor stays plain text.
var highlighters = map[string]*ts.Highlighter{}

func highlightQueryFor(name string, seen map[string]bool) string {
	if seen[name] {
		return ""
	}
	seen[name] = true
	entry := grammars.DetectLanguageByName(name)
	if entry == nil {
		return ""
	}
	query := entry.HighlightQuery
	parent := entry.InheritHighlights
	if parent == "" {
		parent = highlightInherits[entry.Name]
	}
	if parent != "" {
		if parentQuery := highlightQueryFor(parent, seen); parentQuery != "" {
			query = parentQuery + "\n" + query
		}
	}
	return query
}

func highlighterFor(language string) *ts.Highlighter {
	if h, ok := highlighters[language]; ok {
		return h
	}
	var h *ts.Highlighter
	if entry := grammars.DetectLanguageByName(language); entry != nil {
		if lang := entry.Language(); lang != nil {
			if query := strings.TrimSpace(highlightQueryFor(entry.Name, map[string]bool{})); query != "" {
				if built, err := ts.NewHighlighter(lang, query); err == nil {
					h = built
				}
			}
		}
	}
	highlighters[language] = h
	return h
}

// highlight returns capture spans in UTF-16 code-unit offsets, which index
// directly into JavaScript strings.
func highlight(language, source string) highlightJSON {
	out := highlightJSON{Language: language, Spans: []highlightSpanJSON{}}
	h := highlighterFor(language)
	if h == nil {
		return out
	}
	for _, r := range h.HighlightUTF16(utf16.Encode([]rune(source))) {
		out.Spans = append(out.Spans, highlightSpanJSON{
			Start:   int(r.StartCodeUnit),
			End:     int(r.EndCodeUnit),
			Capture: r.Capture,
		})
	}
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

	js.Global().Set("cmHighlight", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 2 {
			return emptyHighlightResult
		}
		b, err := json.Marshal(highlight(args[0].String(), args[1].String()))
		if err != nil {
			return emptyHighlightResult
		}
		return string(b)
	}))

	select {} // keep the Go runtime alive for future calls
}
