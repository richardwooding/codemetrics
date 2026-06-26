# go-codemetrics

[![Go Reference](https://pkg.go.dev/badge/github.com/richardwooding/go-codemetrics.svg)](https://pkg.go.dev/github.com/richardwooding/go-codemetrics)

Per-function **cyclomatic** and **cognitive** complexity for source code, as a
small Go library and CLI.

Most Go tools give you one metric or the other: [`fzipp/gocyclo`][gocyclo] does
cyclomatic, [`uudashr/gocognit`][gocognit] does cognitive. `go-codemetrics`
computes **both in one pass** behind a single API, and is structured so more
languages can be added behind `Parse` without breaking callers.

The Go analyzer is built on the standard library's `go/ast` — **zero external
dependencies**.

## The two metrics

- **Cyclomatic complexity** (McCabe) counts decision points flatly: `1 + one
  per branch` (if / for / range / case / comm-clause / `&&` / `||`). It measures
  the number of independent paths through a function.
- **Cognitive complexity** (SonarSource) weights *nested* control flow more
  heavily, so deeply-nested logic scores higher than a flat sequence with the
  same number of branches. It tracks how hard code is to *understand* rather
  than how many paths it has. The implementation follows the [SonarSource
  specification][sonar]; results match [`uudashr/gocognit`][gocognit].

## Install

```sh
go get github.com/richardwooding/go-codemetrics
```

CLI:

```sh
go install github.com/richardwooding/go-codemetrics/cmd/codemetrics@latest
```

## Library usage

```go
package main

import (
	"fmt"

	codemetrics "github.com/richardwooding/go-codemetrics"
)

func main() {
	src := []byte(`package p
func classify(n int) string {
	if n < 0 {
		return "neg"
	} else if n == 0 {
		return "zero"
	}
	return "pos"
}`)

	fns, err := codemetrics.ParseGo(src)
	if err != nil {
		panic(err)
	}
	for _, f := range fns {
		fmt.Printf("%-12s cyclomatic=%d cognitive=%d lines=%d\n",
			f.QualifiedName(), f.Cyclomatic, *f.Cognitive, f.Lines())
	}
	// classify     cyclomatic=3 cognitive=2 lines=8
}
```

`Parse(language, src)` dispatches by language identifier (`"go"` / `"golang"`
today) and returns a wrapped `ErrUnsupportedLanguage` for anything else.
`SupportedLanguages()` lists what's available.

```go
type FunctionMetrics struct {
	Name       string // bare name, e.g. "Write"
	Receiver   string // receiver type for methods, e.g. "*Buffer"; "" otherwise
	Cyclomatic int
	Cognitive  *int   // nil if unavailable for the language; always set for Go
	StartLine  int    // 1-based, inclusive
	EndLine    int
}
```

`Cognitive` is a pointer so a language without cognitive support is
distinguishable (`nil`) from a genuine zero. For Go it is always populated.

### Already have an AST?

If you've already parsed Go source with `go/parser`, compute either metric for
a single declaration without re-parsing:

```go
func Cyclomatic(body *ast.BlockStmt) int // 1 + branch points
func Cognitive(fn *ast.FuncDecl) int     // SonarSource cognitive complexity
```

Both are nil-safe (return 0). This mirrors the AST-level entry points of
[`gocyclo`][gocyclo] and [`gocognit`][gocognit].

Parsing is best-effort: input that still yields a partial syntax tree is
tolerated and metrics are computed for every recovered function; only a total
parse failure returns an error.

## CLI usage

```sh
# Top 10 functions by cognitive complexity across a tree
codemetrics -top 10 ./...            # (pass directories or files)
codemetrics -top 10 internal/

# Sort by cyclomatic instead, only show the gnarly ones
codemetrics -sort cyclomatic -min 15 .

# JSON for tooling / CI
codemetrics -json ./mypkg | jq '.[] | select(.cognitive > 20)'

# Read from stdin
cat foo.go | codemetrics
```

```
COGNITIVE  CYCLOMATIC  LINES  FUNCTION            LOCATION
83         35          148    BuildCodeGraph      codegraph.go:172
74         39          175    UnusedExports       unused_exports.go:179
...
```

Directories are walked recursively for `.go` files, skipping `vendor`,
`testdata`, and dot-directories.

Flags: `-sort cognitive|cyclomatic`, `-top N`, `-min N`, `-json`.

## Roadmap

The API is shaped for multi-language support: `Parse(language, src)` and the
nil-able `Cognitive` field exist so additional languages can be added without a
breaking change. A tree-sitter backend covering Python / JavaScript /
TypeScript / Java / Rust / C / C++ / C# / Kotlin / PHP / Ruby / Scala and more
is planned.

## License

MIT — see [LICENSE](LICENSE).

[gocyclo]: https://github.com/fzipp/gocyclo
[gocognit]: https://github.com/uudashr/gocognit
[sonar]: https://www.sonarsource.com/docs/CognitiveComplexity.pdf
