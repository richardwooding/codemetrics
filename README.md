# codemetrics

[![Go Reference](https://pkg.go.dev/badge/github.com/richardwooding/codemetrics.svg)](https://pkg.go.dev/github.com/richardwooding/codemetrics)

Per-function **cyclomatic** and **cognitive** complexity across **17
languages** — a **CLI** and **GitHub Action** that gate pull requests on
complexity, plus a Go library.

It computes **both metrics in one pass**, where most tools give you just one
(for Go, [`fzipp/gocyclo`][gocyclo] does cyclomatic and
[`uudashr/gocognit`][gocognit] does cognitive). Go is analyzed with the standard
library's `go/ast` and 16 more languages via
[tree-sitter](#other-languages-tree-sitter) — both backends return the same
`FunctionMetrics`.

**Works with:**

![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)
![Python](https://img.shields.io/badge/Python-3776AB?style=flat-square&logo=python&logoColor=white)
![JavaScript](https://img.shields.io/badge/JavaScript-F7DF1E?style=flat-square&logo=javascript&logoColor=black)
![TypeScript](https://img.shields.io/badge/TypeScript-3178C6?style=flat-square&logo=typescript&logoColor=white)
![Java](https://img.shields.io/badge/Java-007396?style=flat-square&logo=openjdk&logoColor=white)
![Rust](https://img.shields.io/badge/Rust-000000?style=flat-square&logo=rust&logoColor=white)
![C](https://img.shields.io/badge/C-A8B9CC?style=flat-square&logo=c&logoColor=black)
![C++](https://img.shields.io/badge/C%2B%2B-00599C?style=flat-square&logo=cplusplus&logoColor=white)
![C#](https://img.shields.io/badge/C%23-239120?style=flat-square&logo=dotnet&logoColor=white)
![Kotlin](https://img.shields.io/badge/Kotlin-7F52FF?style=flat-square&logo=kotlin&logoColor=white)
![PHP](https://img.shields.io/badge/PHP-777BB4?style=flat-square&logo=php&logoColor=white)
![Ruby](https://img.shields.io/badge/Ruby-CC342D?style=flat-square&logo=ruby&logoColor=white)
![Scala](https://img.shields.io/badge/Scala-DC322F?style=flat-square&logo=scala&logoColor=white)
![R](https://img.shields.io/badge/R-276DC3?style=flat-square&logo=r&logoColor=white)
![MATLAB](https://img.shields.io/badge/MATLAB-0076A8?style=flat-square)
![Perl](https://img.shields.io/badge/Perl-39457E?style=flat-square&logo=perl&logoColor=white)
![Swift](https://img.shields.io/badge/Swift-F05138?style=flat-square&logo=swift&logoColor=white)

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

CLI — Homebrew (macOS):

```sh
brew install --cask richardwooding/tap/codemetrics
```

The cask installs a pre-built binary. Since it isn't notarized, the cask strips
the macOS quarantine attribute on install so it runs without a Gatekeeper
prompt.

CLI — `go install` (any platform), or download a binary from the
[releases page](https://github.com/richardwooding/codemetrics/releases):

```sh
go install github.com/richardwooding/codemetrics/cmd/codemetrics@latest
```

Library:

```sh
go get github.com/richardwooding/codemetrics
```

## GitHub Action (PR complexity gate)

This repo ships a composite Action that runs the `--diff` gate on pull requests:
it installs the released `codemetrics` binary and fails the check when a function
the PR **touches** exceeds your threshold. No Docker image — it just downloads
the binary for the runner.

```yaml
name: complexity
on: pull_request
jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0 # required: full history for the diff's merge-base
      - uses: richardwooding/codemetrics@v0.8.0
        with:
          max-cognitive: "15"
```

Optionally upload SARIF so findings show up in the PR's Files-changed view:

```yaml
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: richardwooding/codemetrics@v0.8.0
        with:
          max-cognitive: "15"
          sarif-file: codemetrics.sarif
      - if: always() # upload even when the gate fails
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: codemetrics.sarif
    permissions:
      contents: read
      security-events: write # needed for the SARIF upload
```

**Inputs:** `paths` (default `.`), `max-cognitive` (default `15`),
`max-cyclomatic` (default `0`), `base-ref` (default the PR base branch),
`baseline`, `sarif-file`, `fail-on-findings` (default `true`),
`codemetrics-version` (default `latest`). Runs on Linux and macOS runners.

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
maps to no supported language are ignored. **Vendored and minified files**
(bundled libraries, `*.min.js`, and content that looks minified — via
[`projectdetect`][projectdetect]) are skipped too, so a stray `jquery.min.js`
under `docs/` doesn't pollute the report; pass `--include-vendored` to analyze
them anyway. (Explicitly-named files and stdin are always analyzed.)

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

### Ignoring a function

Opt an intentionally-complex function out of the gate with a comment directive —
codemetrics honours the ones you may already use with `gocyclo` / `gocognit` /
golangci-lint, plus its own cross-language `codemetrics:ignore`. Put the
directive on the line directly above the function (or trailing its declaration):

```go
//gocyclo:ignore
func bigStateMachine() { … }      // won't fail the gate

//nolint:gocognit
func alsoFine() { … }
```

```python
# codemetrics:ignore
def parser(): ...                 # any language, native comment syntax
```

| Directive | Suppresses | Languages |
|-----------|-----------|-----------|
| `codemetrics:ignore` | both metrics | all |
| `codemetrics:ignore cognitive` / `cyclomatic` | the named metric(s) | all |
| `gocyclo:ignore` | both metrics | Go |
| `nolint` / `nolint:all` | both metrics | Go |
| `nolint:gocyclo` / `nolint:gocognit` | cyclomatic / cognitive | Go |

An ignored function **still appears** in the table/JSON with its metrics (marked
`(ignored: …)`), but produces no finding — so it never fails the gate, lands in
SARIF, or enters a baseline. The metric **libraries** are unchanged and still
match [`gocyclo`][gocyclo] / [`gocognit`][gocognit]; the directive is applied by
the CLI, exactly as those tools do.

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

On GitHub, the [Action](#github-action-pr-complexity-gate) above wires this up
for you; to run the CLI directly in a workflow instead:

```yaml
- run: git fetch origin ${{ github.base_ref }}
- run: codemetrics --diff origin/${{ github.base_ref }}...HEAD --max-cognitive 15 .
```

> The CLI embeds the tree-sitter grammars (~22 MB binary). The **library**
> packages stay dependency-light — this weight lives only in the `codemetrics`
> command.

## Library usage

```go
package main

import (
	"fmt"

	codemetrics "github.com/richardwooding/codemetrics"
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
	Cognitive  *int   // nil if a language reports no cognitive score; set for all supported today
	StartLine  int    // 1-based, inclusive
	EndLine    int
}
```

`Cognitive` is a pointer so a language that reports no cognitive score is
distinguishable (`nil`) from a genuine zero — though every language supported
today populates it.

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

## Other languages (tree-sitter)

The subpackage [`github.com/richardwooding/codemetrics/treesitter`](./treesitter)
computes the same metrics for **16 more languages** using the pure-Go
tree-sitter runtime [`gotreesitter`][gotreesitter]:

> Python · JavaScript · TypeScript · Java · Rust · C · C++ · C# · Kotlin · PHP ·
> Ruby · Scala · R · MATLAB · Perl · Swift

```go
import "github.com/richardwooding/codemetrics/treesitter"

fns, err := treesitter.Parse("python", src) // -> []codemetrics.FunctionMetrics
```

It returns the same `FunctionMetrics` type, so the two backends are
interchangeable. Both cyclomatic and cognitive complexity are computed for every
one of these languages.

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
