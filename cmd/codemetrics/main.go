// Command codemetrics reports per-function cyclomatic and cognitive complexity
// for source files, with table/JSON/SARIF output and a baseline quality gate.
//
// Usage:
//
//	codemetrics [flags] path...
//
// Each path is a source file or a directory (walked recursively, skipping
// build-artefact and dot-directories). Files are routed to a language backend
// by extension — Go via go/ast, 16 more languages via tree-sitter. With no
// paths, source is read from stdin (Go by default; override with --lang).
//
// Thresholds turn the tool into a quality gate: a function whose cognitive or
// cyclomatic complexity exceeds --max-cognitive / --max-cyclomatic becomes a
// finding. With --baseline, findings recorded in the baseline file are
// suppressed and the command exits non-zero only when a NEW finding appears.
package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

// version stamps the SARIF tool.driver; overridden at build time via -ldflags.
var version = "dev"

// CLI is the Kong-parsed command line.
type CLI struct {
	Version kong.VersionFlag `help:"Print the version and exit."`

	Paths []string `arg:"" optional:"" name:"path" help:"Files or directories to analyze; reads stdin when none are given."`

	Sort   string `help:"Sort key for display: cognitive or cyclomatic." enum:"cognitive,cyclomatic" default:"cognitive"`
	Top    int    `help:"Show only the top N rows (0 = all)."`
	Min    int    `help:"Only show functions whose sort metric is >= this."`
	Format string `short:"f" help:"Output format: table, json, or sarif." enum:"table,json,sarif" default:"table"`
	Lang   string `help:"Force a language id for all inputs instead of detecting by extension (also sets the language for stdin)."`

	Diff string `help:"PR mode: only consider functions touched by 'git diff <ref>' (e.g. origin/main...HEAD). Filters display and gate to changed code."`

	IncludeVendored bool `help:"Analyze vendored/minified files too. By default they are skipped in directory walks (node_modules, *.min.js, bundled libraries, minified content)."`

	MaxCognitive  int `help:"Flag functions whose cognitive complexity exceeds this (0 = disabled)."`
	MaxCyclomatic int `help:"Flag functions whose cyclomatic complexity exceeds this (0 = disabled)."`

	Baseline      string `type:"path" help:"Baseline file: findings recorded in it are suppressed; exit non-zero only on new findings."`
	WriteBaseline string `type:"path" help:"Write current findings to this baseline file and exit 0."`
}

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("codemetrics"),
		kong.Description("Per-function cyclomatic and cognitive complexity, with SARIF output and a baseline quality gate."),
		kong.UsageOnError(),
		kong.Vars{"version": version},
	)
	code, err := run(cli)
	if err != nil {
		fmt.Fprintln(os.Stderr, "codemetrics:", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

// run executes the CLI and returns a process exit code. The gate exits 1 only
// when thresholds are set and at least one non-suppressed finding remains.
func run(cli CLI) (int, error) {
	rows, err := collect(cli.Paths, cli.Lang, !cli.IncludeVendored)
	if err != nil {
		return 1, err
	}

	// PR mode: restrict everything downstream to functions the diff touches, so
	// only findings actually in the change are reported and gated.
	if cli.Diff != "" {
		changed, err := changedLines(cli.Diff)
		if err != nil {
			return 1, err
		}
		rows = filterToDiff(rows, changed)
	}

	sortRows(rows, cli.Sort)
	display := applyMinTop(rows, cli.Sort, cli.Min, cli.Top)

	findings := computeFindings(rows, cli.MaxCognitive, cli.MaxCyclomatic)
	gateActive := cli.MaxCognitive > 0 || cli.MaxCyclomatic > 0

	// --write-baseline records the current findings and exits, never gating.
	if cli.WriteBaseline != "" {
		if err := writeBaseline(cli.WriteBaseline, findings); err != nil {
			return 1, err
		}
		fmt.Fprintf(os.Stderr, "codemetrics: wrote %d finding(s) to %s\n", len(findings), cli.WriteBaseline)
		return 0, nil
	}

	// Partition findings into those suppressed by the baseline and the new
	// (active) ones the gate cares about.
	active := findings
	if cli.Baseline != "" {
		base, err := loadBaseline(cli.Baseline)
		if err != nil {
			return 1, err
		}
		active = active[:0:0]
		for _, f := range findings {
			if _, suppressed := base[f.key()]; !suppressed {
				active = append(active, f)
			}
		}
	}

	switch cli.Format {
	case "json":
		if err := emitJSON(os.Stdout, display); err != nil {
			return 1, err
		}
	case "sarif":
		if err := emitSARIF(os.Stdout, active, cli.MaxCognitive, cli.MaxCyclomatic); err != nil {
			return 1, err
		}
	default:
		printTable(os.Stdout, display)
	}

	if gateActive {
		if cli.Baseline != "" {
			fmt.Fprintf(os.Stderr, "codemetrics: %d new finding(s), %d suppressed by baseline\n", len(active), len(findings)-len(active))
		} else {
			fmt.Fprintf(os.Stderr, "codemetrics: %d finding(s)\n", len(active))
		}
		if len(active) > 0 {
			return 1, nil
		}
	}
	return 0, nil
}
