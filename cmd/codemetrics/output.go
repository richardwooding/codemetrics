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

	// One row's cells: plain text (for width) alongside its colored display form.
	type rowCells struct {
		cogPlain, cogShown string
		cyc                string
		lines              string
		fnPlain, fnShown   string
		locPlain, locShown string
	}

	heads := [5]string{"COGNITIVE", "CYCLOMATIC", "LINES", "FUNCTION", "LOCATION"}
	widths := [5]int{}
	for i, h := range heads {
		widths[i] = len(h)
	}

	// Single pass: build cells and grow column widths from the uncolored text.
	data := make([]rowCells, 0, len(rows))
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
		cyc := strconv.Itoa(r.Cyclomatic)
		lines := strconv.Itoa(r.EndLine - r.StartLine + 1)
		loc := r.File + ":" + strconv.Itoa(r.StartLine)

		for i, n := range [5]int{len(cog), len(cyc), len(lines), len(fnPlain), len(loc)} {
			if n > widths[i] {
				widths[i] = n
			}
		}
		data = append(data, rowCells{
			cogPlain: cog, cogShown: pal.cognitive(cog),
			cyc:     cyc,
			lines:   lines,
			fnPlain: fnPlain, fnShown: fnShown,
			locPlain: loc, locShown: pal.location(loc),
		})
	}

	var b strings.Builder
	pad := func(n int) { b.WriteString(strings.Repeat(" ", n+gap)) }

	for i, h := range heads {
		b.WriteString(pal.header(h))
		if i < len(heads)-1 {
			pad(widths[i] - len(h))
		}
	}
	b.WriteByte('\n')

	for _, c := range data {
		b.WriteString(c.cogShown)
		pad(widths[0] - len(c.cogPlain))
		b.WriteString(c.cyc)
		pad(widths[1] - len(c.cyc))
		b.WriteString(c.lines)
		pad(widths[2] - len(c.lines))
		b.WriteString(c.fnShown)
		pad(widths[3] - len(c.fnPlain))
		b.WriteString(c.locShown) // last column: no trailing padding
		b.WriteByte('\n')
	}
	_, _ = io.WriteString(w, b.String())
}
