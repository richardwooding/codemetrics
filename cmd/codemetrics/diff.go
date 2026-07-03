package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// lineRange is an inclusive, 1-based span of lines on the new (post-change)
// side of a diff.
type lineRange struct {
	start, end int
}

// changedLines runs `git diff <base>` and returns, keyed by absolute file path,
// the added/modified line ranges on the new side — i.e. the lines the change
// actually touches. base is passed through to git verbatim, so callers can use
// a rev ("origin/main"), a range ("origin/main...HEAD"), or a commit.
//
// --unified=0 makes each hunk header's new-side range exactly the changed
// lines, so parsing headers alone is sufficient; pure deletions (new count 0)
// contribute nothing.
func changedLines(base string) (map[string][]lineRange, error) {
	root, err := gitRoot()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("git", "diff", "--unified=0", "--no-color", base)
	cmd.Dir = root
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("git diff %s: %s", base, msg)
	}
	return parseDiff(root, out.Bytes()), nil
}

// gitRoot returns the absolute path to the enclosing git working tree root.
func gitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository (git rev-parse --show-toplevel failed)")
	}
	return strings.TrimSpace(string(out)), nil
}

// parseDiff extracts new-side changed line ranges from unified diff output,
// keying by absolute path (root joined with the diff's "b/" path).
func parseDiff(root string, diff []byte) map[string][]lineRange {
	changed := map[string][]lineRange{}
	var curFile string
	sc := bufio.NewScanner(bytes.NewReader(diff))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "+++ "):
			p := strings.TrimPrefix(line, "+++ ")
			if p == "/dev/null" {
				curFile = "" // deleted file: nothing on the new side
				continue
			}
			p = strings.TrimPrefix(p, "b/")
			curFile = filepath.Join(root, p)
		case strings.HasPrefix(line, "@@"):
			if curFile == "" {
				continue
			}
			if r, ok := parseHunkNewRange(line); ok {
				changed[curFile] = append(changed[curFile], r)
			}
		}
	}
	return changed
}

// parseHunkNewRange reads the new-side range from a hunk header of the form
// "@@ -a,b +c,d @@ ...". Returns false for a hunk with no new lines (d == 0).
func parseHunkNewRange(header string) (lineRange, bool) {
	plus := strings.IndexByte(header, '+')
	if plus < 0 {
		return lineRange{}, false
	}
	tok := header[plus+1:]
	if sp := strings.IndexByte(tok, ' '); sp >= 0 {
		tok = tok[:sp]
	}
	start, count := 0, 1
	if comma := strings.IndexByte(tok, ','); comma >= 0 {
		start, _ = strconv.Atoi(tok[:comma])
		count, _ = strconv.Atoi(tok[comma+1:])
	} else {
		start, _ = strconv.Atoi(tok)
	}
	if start <= 0 || count <= 0 {
		return lineRange{}, false
	}
	return lineRange{start: start, end: start + count - 1}, true
}

// filterToDiff keeps only rows whose function line span overlaps a changed line
// range in the same file, matched by absolute path — the functions actually
// touched by the diff.
func filterToDiff(rows []row, changed map[string][]lineRange) []row {
	out := make([]row, 0, len(rows))
	for _, r := range rows {
		abs, err := filepath.Abs(r.File)
		if err != nil {
			continue
		}
		ranges, ok := changed[abs]
		if !ok {
			continue
		}
		if overlapsAny(r.StartLine, r.EndLine, ranges) {
			out = append(out, r)
		}
	}
	return out
}

// overlapsAny reports whether [start,end] intersects any of ranges (all
// inclusive, 1-based).
func overlapsAny(start, end int, ranges []lineRange) bool {
	for _, rg := range ranges {
		if start <= rg.end && rg.start <= end {
			return true
		}
	}
	return false
}
