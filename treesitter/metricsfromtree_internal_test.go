package treesitter

import "testing"

// MetricsFromTree (the shared-parse entry point) must produce exactly what
// Parse produces, when handed the same tree + spans Parse would have built.
func TestMetricsFromTreeMatchesParse(t *testing.T) {
	cases := []struct{ language, src string }{
		{"python", "def branchy(x):\n    if x > 0:\n        for i in range(x):\n            if i % 2 == 0:\n                return i\n    return 0\n\ndef flat(a):\n    return a\n"},
		{"javascript", "function f(n) {\n  if (n === 1) { return 1; }\n  else if (n === 2) { return 2; }\n  else { return 3; }\n}\n"},
		{"rust", "fn branchy(x: i32) -> i32 {\n    if x > 0 {\n        for i in 0..x {\n            if i % 2 == 0 { return i; }\n        }\n    }\n    return 0;\n}\n"},
		{"swift", "func f(_ x: Int) -> Int {\n  if x > 0 { return 1 } else if x < 0 { return 2 }\n  return 0\n}\n"}, // else-if: cognitive spec via elseDirectIf
	}
	for _, tc := range cases {
		t.Run(tc.language, func(t *testing.T) {
			want, err := Parse(tc.language, []byte(tc.src))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			ls := langFor(tc.language)
			if ls == nil {
				t.Fatalf("no langState for %s", tc.language)
			}
			tree, err := ls.pool.Parse([]byte(tc.src))
			if err != nil || tree == nil {
				t.Fatalf("parse: %v", err)
			}
			spans := collectFuncSpans(ls, tree, []byte(tc.src))
			got := MetricsFromTree(tc.language, tree, ls.lang, spans)

			if len(got) != len(want) {
				t.Fatalf("len(MetricsFromTree)=%d, len(Parse)=%d", len(got), len(want))
			}
			for i := range got {
				g, w := got[i], want[i]
				if g.Name != w.Name || g.Cyclomatic != w.Cyclomatic || g.StartLine != w.StartLine || g.EndLine != w.EndLine {
					t.Errorf("[%d] = %+v, want %+v", i, g, w)
				}
				switch {
				case g.Cognitive == nil && w.Cognitive == nil:
				case g.Cognitive != nil && w.Cognitive != nil:
					if *g.Cognitive != *w.Cognitive {
						t.Errorf("[%d] cognitive = %d, want %d", i, *g.Cognitive, *w.Cognitive)
					}
				default:
					t.Errorf("[%d] cognitive nil-mismatch: got %v want %v", i, g.Cognitive, w.Cognitive)
				}
			}
		})
	}
}

// Duplicate spans (same byte range twice — a grammar's tags query and a
// supplemental query matching the same definition) must each receive the full
// metrics, not have one entry silently zeroed by the exact-range span lookup
// in the cognitive walk.
func TestMetricsFromTreeDuplicateSpans(t *testing.T) {
	src := []byte("func f(_ x: Int) -> Int {\n  if x > 0 { return 1 }\n  return 0\n}\n")
	ls := langFor("swift")
	if ls == nil {
		t.Fatal("no langState for swift")
	}
	tree, err := ls.pool.Parse(src)
	if err != nil || tree == nil {
		t.Fatalf("parse: %v", err)
	}
	spans := collectFuncSpans(ls, tree, src)
	if len(spans) != 1 {
		t.Fatalf("collectFuncSpans = %d spans, want 1", len(spans))
	}
	got := MetricsFromTree("swift", tree, ls.lang, []Span{spans[0], spans[0]})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for i, m := range got {
		if m.Cyclomatic != 2 {
			t.Errorf("[%d] Cyclomatic = %d, want 2", i, m.Cyclomatic)
		}
		if m.Cognitive == nil {
			t.Errorf("[%d] Cognitive = nil, want 1", i)
		} else if *m.Cognitive != 1 {
			t.Errorf("[%d] Cognitive = %d, want 1", i, *m.Cognitive)
		}
	}
}
