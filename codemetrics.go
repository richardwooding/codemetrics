package codemetrics

import (
	"errors"
	"fmt"
	"strings"
)

// ErrUnsupportedLanguage is returned by [Parse] for a language with no
// analyzer. Test for it with errors.Is.
var ErrUnsupportedLanguage = errors.New("codemetrics: unsupported language")

// FunctionMetrics holds the complexity metrics for a single function or
// method, together with its 1-based inclusive line span.
type FunctionMetrics struct {
	// Name is the bare function or method name (e.g. "Write"), without the
	// receiver. For methods, Receiver carries the receiver type.
	Name string
	// Receiver is the receiver type of a method (e.g. "*Buffer" or "Buffer"),
	// or "" for a plain function.
	Receiver string
	// Cyclomatic is the McCabe cyclomatic complexity (1 + branch points).
	Cyclomatic int
	// Cognitive is the SonarSource cognitive complexity. It is a pointer so a
	// language whose analyzer computes no cognitive score is distinguishable
	// (nil) from a genuine zero; every language supported today populates it.
	Cognitive *int
	// StartLine and EndLine are the 1-based inclusive line span of the
	// function declaration.
	StartLine int
	EndLine   int
}

// Lines returns the number of source lines the function spans (inclusive).
func (m FunctionMetrics) Lines() int {
	if m.EndLine < m.StartLine {
		return 0
	}
	return m.EndLine - m.StartLine + 1
}

// QualifiedName returns the receiver-qualified name for a method
// (e.g. "(*Buffer).Write") or the bare name for a plain function.
func (m FunctionMetrics) QualifiedName() string {
	if m.Receiver == "" {
		return m.Name
	}
	return "(" + m.Receiver + ")." + m.Name
}

// SupportedLanguages returns the language identifiers accepted by [Parse].
func SupportedLanguages() []string {
	return []string{"go"}
}

// Parse computes complexity metrics for every function in src, dispatching on
// language. Recognised identifiers: "go" (alias "golang"). Unknown languages
// return a wrapped [ErrUnsupportedLanguage].
//
// Parsing is best-effort: input that still yields a partial syntax tree is
// tolerated and metrics are computed for every recovered function. A hard
// parse failure returns a non-nil error and no metrics.
func Parse(language string, src []byte) ([]FunctionMetrics, error) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "go", "golang":
		return ParseGo(src)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedLanguage, language)
	}
}
