package main

import (
	"encoding/json"
	"io"
	"sort"
	"strconv"
	"strings"
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

// printTable renders rows as an aligned table, optionally colorized via pal.
// Columns are padded manually (not text/tabwriter) because tabwriter counts the
// bytes of ANSI escape sequences as visible width and would misalign colored
// cells. Widths are measured from the uncolored text; color is applied on top.
func printTable(w io.Writer, rows []row, pal palette) {
	const gap = 2
	heads := []string{"COGNITIVE", "CYCLOMATIC", "LINES", "FUNCTION", "LOCATION"}

	// cell holds a column's plain text (for width) and its display form (colored).
	type cell struct{ plain, shown string }
	data := make([][]cell, 0, len(rows))
	for _, r := range rows {
		cog := "-"
		if r.Cognitive != nil {
			cog = strconv.Itoa(*r.Cognitive)
		}
		fnPlain, fnShown := r.Function, r.Function
		if len(r.Ignored) > 0 {
			// Show why an over-threshold function won't gate (short metric names).
			short := strings.NewReplacer(ruleCognitive, "cognitive", ruleCyclomatic, "cyclomatic").Replace(strings.Join(r.Ignored, ","))
			cue := " (ignored: " + short + ")"
			fnPlain += cue
			fnShown += pal.muted(cue)
		}
		loc := r.File + ":" + strconv.Itoa(r.StartLine)
		cyc := strconv.Itoa(r.Cyclomatic)
		lines := strconv.Itoa(r.EndLine - r.StartLine + 1)
		data = append(data, []cell{
			{cog, pal.cognitive(cog)},
			{cyc, cyc},
			{lines, lines},
			{fnPlain, fnShown},
			{loc, pal.location(loc)},
		})
	}

	widths := make([]int, len(heads))
	for i, h := range heads {
		widths[i] = len(h)
	}
	for _, cells := range data {
		for i, c := range cells {
			if len(c.plain) > widths[i] {
				widths[i] = len(c.plain)
			}
		}
	}

	var b strings.Builder
	writeRow := func(plains, showns []string) {
		for i := range plains {
			b.WriteString(showns[i])
			if i < len(plains)-1 { // no trailing padding on the last column
				b.WriteString(strings.Repeat(" ", widths[i]-len(plains[i])+gap))
			}
		}
		b.WriteByte('\n')
	}
	hp := make([]string, len(heads))
	hs := make([]string, len(heads))
	for i, h := range heads {
		hp[i] = h
		hs[i] = pal.header(h)
	}
	writeRow(hp, hs)
	for _, cells := range data {
		p := make([]string, len(cells))
		s := make([]string, len(cells))
		for i, c := range cells {
			p[i], s[i] = c.plain, c.shown
		}
		writeRow(p, s)
	}
	_, _ = io.WriteString(w, b.String())
}
