// Package treesitter computes cyclomatic and cognitive complexity for the
// non-Go languages that go-codemetrics supports, using the pure-Go tree-sitter
// runtime github.com/odvcencio/gotreesitter and its bundled grammars.
//
// It is a separate package from the module root so the root
// (github.com/richardwooding/go-codemetrics) stays dependency-free: programs
// that only analyse Go never compile gotreesitter or embed any grammars.
//
// [Parse] is the simple entry point — give it source, get metrics. [MetricsFromTree]
// is for callers that have already parsed the source with gotreesitter (e.g. a
// symbol extractor): it computes the metrics over that existing tree, so symbols
// and metrics share a single parse.
//
// Grammars are embedded at build time. A plain build embeds every bundled
// grammar (~22 MB); to embed only the languages you use, build with the
// gotreesitter subset tags, e.g.
//
//	-tags 'grammar_subset grammar_subset_python grammar_subset_rust'
//
// Cognitive complexity follows the SonarSource specification and is computed for
// every supported language. Each has a cognitive spec, so Cognitive is always
// populated; the type still allows nil (with Cyclomatic reported) should a
// future language ship without one.
package treesitter

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	ts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"

	codemetrics "github.com/richardwooding/go-codemetrics"
)

// parseTimeoutMicros caps a single tree-sitter parse. A pathological grammar
// parse (notably Swift) can run for minutes on a small file and isn't
// cancellable; this bounds it so the source yields no metrics rather than
// hanging. 5 s is far above any healthy parse (milliseconds).
const parseTimeoutMicros = 5_000_000

// Span identifies a function/method definition by name, byte range, and 1-based
// inclusive line range. It is the input [MetricsFromTree] needs to attribute
// metrics to functions — a caller that has already parsed the source and located
// its functions supplies these so no second parse is required.
type Span struct {
	Name               string
	StartByte, EndByte uint32
	StartLine, EndLine int
}

// langState holds the concurrent-safe machinery for one language: a ParserPool
// (safe for concurrent Parse) plus the queries [Parse] needs to find function
// spans. Built once per language on first use.
type langState struct {
	pool      *ts.ParserPool
	lang      *ts.Language
	tagsQuery *ts.Query // bundled grammar tags; function spans for most languages
	spanQuery *ts.Query // supplemental @func.def/@func.name; nil when none
}

var (
	cacheMu sync.Mutex
	cache   = map[string]*langState{} // language -> *langState; nil = unsupported

	decisionMu    sync.Mutex
	decisionCache = map[*ts.Language]*ts.Query{} // per-grammar compiled decision query
)

// langFor lazily builds and caches the tree-sitter machinery for a language,
// or returns nil when the language isn't supported or its grammar is
// unavailable in this build.
func langFor(language string) *langState {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if ls, ok := cache[language]; ok {
		return ls
	}
	ls := buildLangState(language)
	cache[language] = ls
	return ls
}

func buildLangState(language string) *langState {
	sample, ok := detectFile[language]
	if !ok {
		return nil
	}
	entry := grammars.DetectLanguage(sample)
	if entry == nil {
		return nil
	}
	lang := entry.Language()
	if lang == nil {
		return nil
	}
	ls := &langState{lang: lang, pool: ts.NewParserPool(lang, ts.WithParserPoolTimeoutMicros(parseTimeoutMicros))}
	if tagsSrc := grammars.ResolveTagsQuery(*entry); tagsSrc != "" {
		if q, err := ts.NewQuery(tagsSrc, lang); err == nil {
			ls.tagsQuery = q
		}
	}
	if src := funcSpanQuery[language]; src != "" {
		if q, err := ts.NewQuery(src, lang); err == nil {
			ls.spanQuery = q
		}
	}
	return ls
}

// decisionQueryFor returns the compiled cyclomatic decision query for language,
// compiled against lang and cached per grammar (gotreesitter returns a singleton
// *ts.Language per language, so this is effectively per-language). Returns nil
// when the language has no decision query or it fails to compile.
func decisionQueryFor(language string, lang *ts.Language) *ts.Query {
	src := decisionQuery[language]
	if src == "" || lang == nil {
		return nil
	}
	decisionMu.Lock()
	defer decisionMu.Unlock()
	if q, ok := decisionCache[lang]; ok {
		return q
	}
	q, err := ts.NewQuery(src, lang)
	if err != nil {
		q = nil
	}
	decisionCache[lang] = q
	return q
}

// SupportedLanguages returns the language identifiers Parse accepts, sorted.
func SupportedLanguages() []string {
	out := make([]string, 0, len(detectFile))
	for l := range detectFile {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// Parse computes cyclomatic and cognitive complexity for every function in src,
// for one of the supported non-Go languages (see [SupportedLanguages]).
//
// An unsupported or unavailable language returns a wrapped
// [codemetrics.ErrUnsupportedLanguage]. Parsing is best-effort: source that only
// partially parses yields metrics for the functions recovered; source whose
// parse times out or fails yields an empty slice and a nil error.
func Parse(language string, src []byte) ([]codemetrics.FunctionMetrics, error) {
	ls := langFor(language)
	if ls == nil {
		return nil, fmt.Errorf("%w: %q", codemetrics.ErrUnsupportedLanguage, language)
	}
	tree, err := ls.pool.Parse(src)
	if err != nil || tree == nil {
		return nil, nil // timeout / parse failure → best-effort empty
	}
	spans := collectFuncSpans(ls, tree, src)
	if len(spans) == 0 {
		return nil, nil
	}
	return MetricsFromTree(language, tree, ls.lang, spans), nil
}

// MetricsFromTree computes cyclomatic and cognitive complexity for each span,
// over an already-parsed tree, without re-parsing. Use it when you have already
// parsed src with gotreesitter (e.g. while extracting symbols) and located its
// functions: pass the parse tree, the *ts.Language it was parsed with, and the
// function spans, and receive metrics index-aligned with spans.
//
// language selects the decision-node set and cognitive spec; it must match the
// grammar the tree was parsed with. An unsupported language yields per-span
// metrics with Cyclomatic 1 (no decision query) and nil Cognitive.
func MetricsFromTree(language string, tree *ts.Tree, lang *ts.Language, spans []Span) []codemetrics.FunctionMetrics {
	if tree == nil || len(spans) == 0 {
		return nil
	}
	decisions := make([]int, len(spans))
	if dq := decisionQueryFor(language, lang); dq != nil {
		for _, m := range dq.Execute(tree) {
			for _, c := range m.Captures {
				if c.Name != "decision" {
					continue
				}
				if i := innermostFuncSpanIndex(spans, c.Node.StartByte()); i >= 0 {
					decisions[i]++
				}
			}
		}
	}
	cognitive := cognitiveComplexity(language, lang, tree, spans)
	out := make([]codemetrics.FunctionMetrics, 0, len(spans))
	for i, s := range spans {
		m := codemetrics.FunctionMetrics{
			Name:       s.Name,
			Cyclomatic: 1 + decisions[i],
			StartLine:  s.StartLine,
			EndLine:    s.EndLine,
		}
		if cognitive != nil && i < len(cognitive) && cognitive[i] != nil {
			m.Cognitive = cognitive[i]
		}
		out = append(out, m)
	}
	return out
}

func newSpan(name string, n *ts.Node) Span {
	return Span{
		Name: name, StartByte: n.StartByte(), EndByte: n.EndByte(),
		StartLine: int(n.StartPoint().Row + 1), EndLine: int(n.EndPoint().Row + 1),
	}
}

// collectFuncSpans gathers function spans from the bundled tags query (most
// languages) plus the supplemental span query (ruby/swift/kotlin/php/perl/r).
func collectFuncSpans(ls *langState, tree *ts.Tree, src []byte) []Span {
	var spans []Span
	if ls.tagsQuery != nil {
		for _, m := range ls.tagsQuery.Execute(tree) {
			var name, kind string
			var defNode *ts.Node
			for _, c := range m.Captures {
				switch {
				case c.Name == "name":
					name = c.Text(src)
				case strings.HasPrefix(c.Name, "definition."):
					kind = c.Name[len("definition."):]
					defNode = c.Node
				}
			}
			if name == "" || defNode == nil {
				continue
			}
			switch kind {
			case "function", "method", "macro", "constructor":
				spans = append(spans, newSpan(name, defNode))
			}
		}
	}
	if ls.spanQuery != nil {
		for _, m := range ls.spanQuery.Execute(tree) {
			var name string
			var defNode *ts.Node
			for _, c := range m.Captures {
				switch c.Name {
				case "func.name":
					name = c.Text(src)
				case "func.def":
					defNode = c.Node
				}
			}
			if name != "" && defNode != nil {
				spans = append(spans, newSpan(name, defNode))
			}
		}
	}
	return spans
}

// innermostFuncSpanIndex returns the index of the smallest function span
// containing pos, or -1 if none does.
func innermostFuncSpanIndex(spans []Span, pos uint32) int {
	best := -1
	bestSize := ^uint32(0)
	for i, s := range spans {
		if pos < s.StartByte || pos >= s.EndByte {
			continue
		}
		if size := s.EndByte - s.StartByte; size < bestSize {
			bestSize = size
			best = i
		}
	}
	return best
}
