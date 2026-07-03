package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
)

// metricOf returns the value a row is ranked/filtered by. Cyclomatic is used
// when it's the chosen key or when the language reports no cognitive value.
func metricOf(r row, sortKey string) int {
	if sortKey == "cyclomatic" || r.Cognitive == nil {
		return r.Cyclomatic
	}
	return *r.Cognitive
}

// sortRows orders rows by the sort metric (descending), then file then start
// line for a stable, deterministic layout.
func sortRows(rows []row, sortKey string) {
	sort.SliceStable(rows, func(i, j int) bool {
		if mi, mj := metricOf(rows[i], sortKey), metricOf(rows[j], sortKey); mi != mj {
			return mi > mj
		}
		if rows[i].File != rows[j].File {
			return rows[i].File < rows[j].File
		}
		return rows[i].StartLine < rows[j].StartLine
	})
}

// applyMinTop filters by the --min threshold and truncates to --top, returning
// the subset to display. It does not mutate the input beyond re-slicing.
func applyMinTop(rows []row, sortKey string, min, top int) []row {
	out := rows
	if min > 0 {
		kept := make([]row, 0, len(out))
		for _, r := range out {
			if metricOf(r, sortKey) >= min {
				kept = append(kept, r)
			}
		}
		out = kept
	}
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	return out
}

func emitJSON(w io.Writer, rows []row) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

func printTable(w io.Writer, rows []row) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "COGNITIVE\tCYCLOMATIC\tLINES\tFUNCTION\tLOCATION")
	for _, r := range rows {
		cog := "-"
		if r.Cognitive != nil {
			cog = fmt.Sprintf("%d", *r.Cognitive)
		}
		lines := r.EndLine - r.StartLine + 1
		_, _ = fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%s:%d\n", cog, r.Cyclomatic, lines, r.Function, r.File, r.StartLine)
	}
	_ = tw.Flush()
}
