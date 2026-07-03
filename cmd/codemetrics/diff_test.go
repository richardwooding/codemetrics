package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseHunkNewRange(t *testing.T) {
	cases := []struct {
		header string
		want   lineRange
		ok     bool
	}{
		{"@@ -1,3 +1,4 @@ func f() {", lineRange{1, 4}, true},
		{"@@ -10 +12 @@", lineRange{12, 12}, true},            // single line, no count
		{"@@ -5,2 +7,0 @@", lineRange{}, false},               // pure deletion (new count 0)
		{"@@ -0,0 +1,10 @@ new file", lineRange{1, 10}, true}, // added file
		{"not a hunk", lineRange{}, false},
	}
	for _, tc := range cases {
		got, ok := parseHunkNewRange(tc.header)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Errorf("parseHunkNewRange(%q) = %+v,%v; want %+v,%v", tc.header, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseDiff(t *testing.T) {
	root := "/repo"
	diff := `diff --git a/src/calc.py b/src/calc.py
index 111..222 100644
--- a/src/calc.py
+++ b/src/calc.py
@@ -3,0 +4,5 @@ def classify(n):
+    for i in range(n):
+        if i:
+            pass
+    return n
+
diff --git a/old.go b/old.go
deleted file mode 100644
index 333..000
--- a/old.go
+++ /dev/null
@@ -1,4 +0,0 @@
-package p
diff --git a/main.go b/main.go
index 444..555 100644
--- a/main.go
+++ b/main.go
@@ -10,2 +10,1 @@ func g() {
-old
-old2
+new
`
	got := parseDiff(root, []byte(diff))
	want := map[string][]lineRange{
		filepath.Join(root, "src/calc.py"): {{4, 8}},
		filepath.Join(root, "main.go"):     {{10, 10}},
		// old.go maps to /dev/null on the new side → excluded entirely.
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDiff =\n%+v\nwant\n%+v", got, want)
	}
}

func TestOverlapsAny(t *testing.T) {
	ranges := []lineRange{{10, 20}, {50, 55}}
	cases := []struct {
		start, end int
		want       bool
	}{
		{1, 5, false},   // before all
		{5, 10, true},   // touches start of first
		{20, 30, true},  // touches end of first
		{21, 49, false}, // gap between ranges
		{40, 60, true},  // spans the second
		{56, 80, false}, // after all
	}
	for _, tc := range cases {
		if got := overlapsAny(tc.start, tc.end, ranges); got != tc.want {
			t.Errorf("overlapsAny(%d,%d) = %v, want %v", tc.start, tc.end, got, tc.want)
		}
	}
}

func TestFilterToDiff(t *testing.T) {
	// Use absolute paths so filepath.Abs is a no-op and keys match.
	fileA, _ := filepath.Abs("pkg/a.go")
	fileB, _ := filepath.Abs("pkg/b.go")
	rows := []row{
		{File: "pkg/a.go", Function: "Touched", StartLine: 8, EndLine: 15},    // overlaps [10,12]
		{File: "pkg/a.go", Function: "Untouched", StartLine: 40, EndLine: 60}, // no overlap
		{File: "pkg/b.go", Function: "OtherFile", StartLine: 1, EndLine: 5},   // file not in diff
	}
	changed := map[string][]lineRange{
		fileA: {{10, 12}},
		// fileB intentionally absent
	}
	_ = fileB
	got := filterToDiff(rows, changed)
	if len(got) != 1 || got[0].Function != "Touched" {
		t.Fatalf("filterToDiff kept %+v; want only Touched", got)
	}
}
