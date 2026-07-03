package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	sarif "github.com/richardwooding/go-sarif"
)

// Rule ids, shared by SARIF output and baseline identity.
const (
	ruleCognitive  = "cognitive-complexity"
	ruleCyclomatic = "cyclomatic-complexity"
)

// finding is one function that exceeds a configured complexity threshold.
type finding struct {
	Rule      string // ruleCognitive | ruleCyclomatic
	File      string
	Function  string
	Value     int // the measured complexity
	Threshold int // the max it exceeded
	StartLine int
	EndLine   int
}

// key is the baseline identity: rule + file + function, deliberately excluding
// line numbers and the measured value so a suppressed finding stays suppressed
// when code shifts lines or its complexity changes slightly.
func (f finding) key() string {
	return f.Rule + "\x00" + f.File + "\x00" + f.Function
}

// metricName is the human word used in messages ("cognitive" / "cyclomatic").
func (f finding) metricName() string {
	if f.Rule == ruleCyclomatic {
		return "cyclomatic"
	}
	return "cognitive"
}

// computeFindings derives findings from rows for whichever thresholds are
// enabled (a zero threshold disables that rule). A single function can produce
// both a cognitive and a cyclomatic finding.
func computeFindings(rows []row, maxCog, maxCyc int) []finding {
	var out []finding
	for _, r := range rows {
		if maxCog > 0 && r.Cognitive != nil && *r.Cognitive > maxCog {
			out = append(out, finding{
				Rule: ruleCognitive, File: r.File, Function: r.Function,
				Value: *r.Cognitive, Threshold: maxCog, StartLine: r.StartLine, EndLine: r.EndLine,
			})
		}
		if maxCyc > 0 && r.Cyclomatic > maxCyc {
			out = append(out, finding{
				Rule: ruleCyclomatic, File: r.File, Function: r.Function,
				Value: r.Cyclomatic, Threshold: maxCyc, StartLine: r.StartLine, EndLine: r.EndLine,
			})
		}
	}
	return out
}

// baselineFile is the on-disk baseline: identity-only entries (no line/value).
type baselineFile struct {
	Version  int             `json:"version"`
	Findings []baselineEntry `json:"findings"`
}

type baselineEntry struct {
	Rule     string `json:"rule"`
	File     string `json:"file"`
	Function string `json:"function"`
}

func (e baselineEntry) key() string {
	return e.Rule + "\x00" + e.File + "\x00" + e.Function
}

// writeBaseline records the identities of findings to path (deduped and sorted
// for stable diffs).
func writeBaseline(path string, findings []finding) error {
	seen := map[string]struct{}{}
	entries := make([]baselineEntry, 0, len(findings))
	for _, f := range findings {
		e := baselineEntry{Rule: f.Rule, File: f.File, Function: f.Function}
		if _, dup := seen[e.key()]; dup {
			continue
		}
		seen[e.key()] = struct{}{}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].File != entries[j].File {
			return entries[i].File < entries[j].File
		}
		if entries[i].Function != entries[j].Function {
			return entries[i].Function < entries[j].Function
		}
		return entries[i].Rule < entries[j].Rule
	})
	data, err := json.MarshalIndent(baselineFile{Version: 1, Findings: entries}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// loadBaseline reads a baseline file into a set of finding identity keys.
func loadBaseline(path string) (map[string]struct{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bf baselineFile
	if err := json.Unmarshal(data, &bf); err != nil {
		return nil, fmt.Errorf("parse baseline %s: %w", path, err)
	}
	set := make(map[string]struct{}, len(bf.Findings))
	for _, e := range bf.Findings {
		set[e.key()] = struct{}{}
	}
	return set, nil
}

// emitSARIF writes findings as a SARIF 2.1.0 run. Only rules whose threshold is
// enabled are declared.
func emitSARIF(w io.Writer, findings []finding, maxCog, maxCyc int) error {
	tool := sarif.Tool{
		Name:           "codemetrics",
		Version:        version,
		InformationURI: "https://github.com/richardwooding/go-codemetrics",
	}
	var rules []sarif.Rule
	if maxCog > 0 {
		rules = append(rules, sarif.Rule{
			ID:          ruleCognitive,
			Name:        "CognitiveComplexity",
			Description: "Function cognitive complexity exceeds the configured maximum.",
		})
	}
	if maxCyc > 0 {
		rules = append(rules, sarif.Rule{
			ID:          ruleCyclomatic,
			Name:        "CyclomaticComplexity",
			Description: "Function cyclomatic complexity exceeds the configured maximum.",
		})
	}
	results := make([]sarif.Result, 0, len(findings))
	for _, f := range findings {
		results = append(results, sarif.Result{
			RuleID:    f.Rule,
			Level:     "warning",
			Message:   fmt.Sprintf("%s has %s complexity %d (max %d)", f.Function, f.metricName(), f.Value, f.Threshold),
			URI:       f.File,
			StartLine: f.StartLine,
			EndLine:   f.EndLine,
		})
	}
	return sarif.Write(w, tool, rules, results)
}
