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
	// Force standard a/ and b/ path prefixes so parsing is immune to a user's
	// diff.noprefix / diff.mnemonicPrefix / custom-prefix git config.
	cmd := exec.Command("git", "diff", "--unified=0", "--no-color", "--src-prefix=a/", "--dst-prefix=b/", base)
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
// keying by canonical (absolute, symlink-resolved) path.
//
// File headers are only trusted between a "diff --git" line and that section's
// first hunk. This matters because with --unified=0 an added source line like
// "++ x" renders as "+++ x" in the body — identical to a new-file header — so
// scanning for "+++ " everywhere would corrupt path tracking. Gating on the
// section state (plus the forced "b/" prefix) makes that impossible.
func parseDiff(root string, diff []byte) map[string][]lineRange {
	changed := map[string][]lineRange{}
	var curFile string
	expectHeader := false
	sc := bufio.NewScanner(bytes.NewReader(diff))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			expectHeader = true // file headers follow, then hunks
			curFile = ""
		case expectHeader && (strings.HasPrefix(line, "+++ b/") || line == "+++ /dev/null"):
			if line == "+++ /dev/null" {
				curFile = "" // deleted file: nothing on the new side
				continue
			}
			curFile = canonicalPath(filepath.Join(root, strings.TrimPrefix(line, "+++ b/")))
		case strings.HasPrefix(line, "@@"):
			expectHeader = false // headers for this section are done
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
		ranges, ok := changed[canonicalPath(r.File)]
		if !ok {
			continue
		}
		if overlapsAny(r.StartLine, r.EndLine, ranges) {
			out = append(out, r)
		}
	}
	return out
}

// canonicalPath returns the absolute, symlink-resolved form of path so the
// paths from git diff and the walked source files compare equal regardless of
// symlinks or casing quirks. Falls back to the absolute (then raw) path when
// the file can't be resolved — e.g. it doesn't exist on disk.
func canonicalPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	if eval, err := filepath.EvalSymlinks(abs); err == nil {
		return eval
	}
	return abs
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
