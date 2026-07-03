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

The CLI is built on [Kong][kong] and is **polyglot**: files are routed to a
language backend by extension via [`projectdetect`][projectdetect] — Go through
`go/ast`, the other 16 languages through the tree-sitter backend.

```sh
# Top 10 functions by cognitive complexity across a tree
codemetrics --top 10 ./...            # (pass directories or files)
codemetrics --top 10 internal/

# Sort by cyclomatic instead, only show the gnarly ones
codemetrics --sort cyclomatic --min 15 .

# JSON for tooling / CI
codemetrics --format json ./mypkg | jq '.[] | select(.cognitive > 20)'

# Read from stdin (Go by default; --lang for anything else)
cat foo.go | codemetrics
cat foo.py | codemetrics --lang python
```

```
COGNITIVE  CYCLOMATIC  LINES  FUNCTION            LOCATION
83         35          148    BuildCodeGraph      codegraph.go:172
74         39          175    UnusedExports       unused_exports.go:179
...
```

Directories are walked recursively, skipping dot-directories, `testdata`, and
each detected project's build-artefact dirs (`vendor`, `node_modules`,
`target`, `__pycache__`, …, resolved by `projectdetect`). Files whose extension
maps to no supported language are ignored.

Flags: `--sort cognitive|cyclomatic`, `--top N`, `--min N`,
`--format table|json|sarif`, `--lang <id>`, plus the quality-gate flags below.

> **Flag style changed.** The CLI now uses GNU-style double-dash flags
> (`--sort`, `--top`, `--min`) and `--json` is replaced by `--format json`.

### Quality gate: SARIF + baseline

Set a threshold and any function above it becomes a **finding**. Findings render
as [SARIF 2.1.0][sarif] (via [`go-sarif`][go-sarif]) for GitHub Code Scanning,
and the process exits non-zero when a finding remains — so it gates CI.

```sh
# Fail the build if any function is too complex; emit SARIF for Code Scanning
codemetrics --format sarif --max-cognitive 15 --max-cyclomatic 20 . > results.sarif
```

To adopt the gate on an existing codebase without fixing all debt first, record
a **baseline**. Baselined findings (matched by rule + file + function, ignoring
line numbers) are suppressed; the gate then fails only on *new* findings — the
same pattern as detekt / semgrep.

```sh
# 1. Record today's findings as the accepted baseline
codemetrics --max-cognitive 15 --write-baseline .codemetrics-baseline.json .

# 2. In CI: pass unless a NEW violation is introduced
codemetrics --max-cognitive 15 --baseline .codemetrics-baseline.json .
```

A GitHub Actions step that uploads the SARIF:

```yaml
- run: codemetrics --format sarif --max-cognitive 15 . > results.sarif
  continue-on-error: true          # let the upload run even when the gate fails
- uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: results.sarif
```

Quality-gate flags: `--max-cognitive N`, `--max-cyclomatic N` (0 = disabled),
`--baseline FILE`, `--write-baseline FILE`.

### PR mode: gate only what changed

`--diff <ref>` restricts the whole run to functions **touched by a git diff**, so
a pull request is judged only on the code it actually changes — not the
pre-existing tree. It parses `git diff <ref>` and keeps a function only when its
line span overlaps a changed (new-side) line, then applies your thresholds to
that subset.

```sh
# Fail the PR only if a function it touches is too complex
codemetrics --diff origin/main...HEAD --max-cognitive 15 .
```

`<ref>` is passed straight to `git diff`, so use whatever expresses "this PR":
`origin/main...HEAD` (changes since the merge base — the usual PR diff),
`origin/main`, or `HEAD~1`. Added files count in full; modified files count only
their changed regions. Combine with `--format sarif` to upload PR-scoped
findings, or with `--baseline` to also suppress known offenders among them.

A GitHub Actions PR gate:

```yaml
- run: git fetch origin ${{ github.base_ref }}
- run: codemetrics --diff origin/${{ github.base_ref }}...HEAD --max-cognitive 15 .
```

> The CLI embeds the tree-sitter grammars (~22 MB binary). The **library**
> packages stay dependency-light — this weight lives only in the `codemetrics`
> command.

## Other languages (tree-sitter)

The subpackage [`github.com/richardwooding/go-codemetrics/treesitter`](./treesitter)
computes the same metrics for **16 more languages** using the pure-Go
tree-sitter runtime [`gotreesitter`][gotreesitter]:

> Python · JavaScript · TypeScript · Java · Rust · C · C++ · C# · Kotlin · PHP ·
> Ruby · Scala · R · MATLAB · Perl · Swift

```go
import "github.com/richardwooding/go-codemetrics/treesitter"

fns, err := treesitter.Parse("python", src) // -> []codemetrics.FunctionMetrics
```

It returns the same `FunctionMetrics` type, so the two backends are
interchangeable. Cognitive complexity is computed for every language except
Swift (whose grammar lacks a stable cognitive spec) — there `Cognitive` is nil
while `Cyclomatic` is still reported.

**Already parsed the tree?** If you've parsed the source with gotreesitter
yourself (e.g. while extracting symbols), compute metrics over that existing
tree — no second parse:

```go
func MetricsFromTree(language string, tree *ts.Tree, lang *ts.Language, spans []Span) []codemetrics.FunctionMetrics
```

This is how [`treesitter-symbols`][tss] returns symbols *and* complexity from a
single parse.

**Dependencies stay opt-in.** The module root is `go/ast`-only and pulls in
nothing; `gotreesitter` and its embedded grammars are compiled in *only* when
you import the `treesitter` subpackage. A plain build of that subpackage embeds
every bundled grammar (~22 MB); to embed only the languages you use, build with
the gotreesitter subset tags:

```sh
go build -tags 'grammar_subset grammar_subset_python grammar_subset_rust' ./...
```

## License

MIT — see [LICENSE](LICENSE).

[gocyclo]: https://github.com/fzipp/gocyclo
[gocognit]: https://github.com/uudashr/gocognit
[sonar]: https://www.sonarsource.com/docs/CognitiveComplexity.pdf
[gotreesitter]: https://github.com/odvcencio/gotreesitter
[kong]: https://github.com/alecthomas/kong
[go-sarif]: https://github.com/richardwooding/go-sarif
[projectdetect]: https://github.com/richardwooding/projectdetect

[tss]: https://github.com/richardwooding/treesitter-symbols
