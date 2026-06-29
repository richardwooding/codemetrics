package treesitter

import (
	ts "github.com/odvcencio/gotreesitter"
)

// Cognitive complexity for the tree-sitter languages, the counterpart to the
// precise go/ast implementation in the parent package. It walks each
// function's parse subtree tracking nesting depth and applies the SonarSource
// increments: a structural node (if / loop / switch / catch / ternary) costs
// 1 + the current nesting and raises the nesting for its body; a continuation
// (else / else-if) costs a flat 1; and each maximal run of like logical
// operators costs 1.
//
// It is enabled per-language via cognitiveSpecs. Languages without an entry
// return no values, so cognitive is reported as unavailable (nil) rather than a
// wrong number. The C-family grammars model `else if` as a nested if in the
// else branch; cognitiveSpec.elseField / elseParentType let the walk recognise
// that shape and charge it the flat else-if cost.
//
// Swift is deliberately omitted (no spec) and must stay that way until the
// upstream grammar is fixed: tree-sitter-swift mis-parses `else if` chains,
// emitting ERROR nodes and degrading the enclosing function_declaration, so any
// cognitive walk over a real function containing an else-if produces a wrong
// number. Single if/else, guard, switch, for, while, repeat, and &&/|| all
// parse clean — but else-if is common enough that a partial spec would
// re-introduce the inconsistency (see file-search-on#491). Tracked upstream at
// github.com/odvcencio/gotreesitter#131; re-add a swift entry only once that
// closes and a real else-if function parses cleanly.

// cognitiveSpec classifies one grammar's nodes for the cognitive walk.
type cognitiveSpec struct {
	// nesting nodes cost 1 + nesting and raise the nesting level for children.
	nesting map[string]bool
	// flat nodes cost a flat 1 (continuations like elif_clause / else_clause).
	flat map[string]bool
	// ifType is the grammar's if node — the only node whose else branch is
	// scanned for an else-if. Empty disables else-if detection.
	ifType string
	// elseField, when set, is the field on an if node holding the else branch;
	// when that field's value is itself an ifType node, it's an `else if`.
	elseField string
	// elseParentType, when set, is the wrapper node holding the else branch; a
	// direct ifType child of that wrapper is an `else if`.
	elseParentType string
}

func nodeSet(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// cognitiveSpecs is the per-language enablement + node classification.
var cognitiveSpecs = map[string]cognitiveSpec{
	"python": {
		nesting: nodeSet("if_statement", "for_statement", "while_statement", "except_clause", "conditional_expression", "match_statement"),
		flat:    nodeSet("elif_clause", "else_clause"),
	},
	"javascript": {
		nesting:   nodeSet("if_statement", "for_statement", "for_in_statement", "for_of_statement", "while_statement", "do_statement", "switch_statement", "catch_clause", "ternary_expression"),
		ifType:    "if_statement",
		elseField: "alternative",
	},
	"typescript": {
		nesting:   nodeSet("if_statement", "for_statement", "for_in_statement", "for_of_statement", "while_statement", "do_statement", "switch_statement", "catch_clause", "ternary_expression"),
		ifType:    "if_statement",
		elseField: "alternative",
	},
	"java": {
		nesting:   nodeSet("if_statement", "for_statement", "enhanced_for_statement", "while_statement", "do_statement", "switch_statement", "switch_expression", "catch_clause", "ternary_expression"),
		ifType:    "if_statement",
		elseField: "alternative",
	},
	"rust": {
		nesting:        nodeSet("if_expression", "while_expression", "for_expression", "loop_expression", "match_expression"),
		ifType:         "if_expression",
		elseParentType: "else_clause",
	},
	"c": {
		nesting:        nodeSet("if_statement", "for_statement", "while_statement", "do_statement", "switch_statement", "conditional_expression"),
		ifType:         "if_statement",
		elseParentType: "else_clause",
	},
	"cpp": {
		nesting:        nodeSet("if_statement", "for_statement", "for_range_loop", "while_statement", "do_statement", "switch_statement", "catch_clause", "conditional_expression"),
		ifType:         "if_statement",
		elseParentType: "else_clause",
	},
	"csharp": {
		nesting:   nodeSet("if_statement", "for_statement", "foreach_statement", "while_statement", "do_statement", "switch_statement", "catch_clause", "conditional_expression"),
		ifType:    "if_statement",
		elseField: "alternative",
	},
	"kotlin": {
		nesting:        nodeSet("if_expression", "for_statement", "while_statement", "do_while_statement", "when_expression", "catch_block"),
		ifType:         "if_expression",
		elseParentType: "control_structure_body",
	},
	"php": {
		nesting: nodeSet("if_statement", "for_statement", "foreach_statement", "while_statement", "do_statement", "switch_statement", "catch_clause", "conditional_expression"),
		flat:    nodeSet("else_if_clause", "else_clause"),
	},
	"ruby": {
		nesting: nodeSet("if", "unless", "while", "until", "for", "case", "rescue", "conditional"),
		flat:    nodeSet("elsif"),
	},
	"scala": {
		nesting:   nodeSet("if_expression", "for_expression", "while_expression", "match_expression", "catch_clause"),
		ifType:    "if_expression",
		elseField: "alternative",
	},
	"r": {
		nesting:   nodeSet("if_statement", "for_statement", "while_statement"),
		ifType:    "if_statement",
		elseField: "alternative",
	},
	"matlab": {
		nesting: nodeSet("if_statement", "for_statement", "while_statement", "switch_statement"),
		flat:    nodeSet("elseif_clause", "else_clause"),
	},
	"perl": {
		nesting: nodeSet("conditional_statement", "loop_statement"),
		flat:    nodeSet("elsif", "else"),
	},
}

// cognitiveComplexity computes cognitive complexity for each function span, or
// returns nil when the language has no spec (cognitive unavailable). The
// result is index-aligned with funcSpans; each entry is a fresh *int. lang must
// be the *ts.Language the tree was parsed with.
func cognitiveComplexity(language string, lang *ts.Language, tree *ts.Tree, funcSpans []Span) []*int {
	spec, ok := cognitiveSpecs[language]
	if !ok {
		return nil
	}
	if tree == nil || len(funcSpans) == 0 {
		return []*int{}
	}
	cog := make([]int, len(funcSpans))
	byRange := make(map[[2]uint32]int, len(funcSpans))
	for i, s := range funcSpans {
		byRange[[2]uint32{s.StartByte, s.EndByte}] = i
	}
	elseIf := map[[2]uint32]bool{}

	var walk func(n *ts.Node, spanIdx, nesting int)
	walk = func(n *ts.Node, spanIdx, nesting int) {
		for i := 0; i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil {
				continue
			}
			if !c.IsNamed() {
				continue
			}
			rng := [2]uint32{c.StartByte(), c.EndByte()}
			if idx, ok := byRange[rng]; ok {
				walk(c, idx, 0)
				continue
			}
			t := c.Type(lang)
			if spec.nesting[t] {
				if elseIf[rng] {
					if spanIdx >= 0 {
						cog[spanIdx]++ // else if: flat cost, no nesting penalty
					}
					tagElseIf(c, t, spec, lang, elseIf)
					walk(c, spanIdx, nesting)
				} else {
					if spanIdx >= 0 {
						cog[spanIdx] += 1 + nesting
					}
					tagElseIf(c, t, spec, lang, elseIf)
					walk(c, spanIdx, nesting+1)
				}
			} else if spec.flat[t] {
				if spanIdx >= 0 {
					cog[spanIdx]++
				}
				walk(c, spanIdx, nesting)
			} else if op := boolOp(c, lang); op != "" {
				if spanIdx >= 0 && !sameRunAsParent(c, op, lang) {
					cog[spanIdx]++
				}
				walk(c, spanIdx, nesting)
			} else {
				walk(c, spanIdx, nesting)
			}
		}
	}
	walk(tree.RootNode(), -1, 0)

	out := make([]*int, len(cog))
	for i := range cog {
		out[i] = &cog[i]
	}
	return out
}

// tagElseIf records, while the walk is AT an if node, the byte range of an
// if-node sitting in its else branch — an `else if`, charged the flat else-if
// cost. Driven from the parent so it never relies on Node.Parent(), which
// gotreesitter routes through hidden wrappers.
func tagElseIf(c *ts.Node, t string, spec cognitiveSpec, lang *ts.Language, set map[[2]uint32]bool) {
	if spec.ifType == "" || t != spec.ifType {
		return
	}
	if spec.elseField != "" {
		if alt := c.ChildByFieldName(spec.elseField, lang); alt != nil && alt.Type(lang) == t {
			set[[2]uint32{alt.StartByte(), alt.EndByte()}] = true
		}
	}
	if spec.elseParentType != "" {
		for i := 0; i < c.ChildCount(); i++ {
			w := c.Child(i)
			if w == nil || w.Type(lang) != spec.elseParentType {
				continue
			}
			for j := 0; j < w.ChildCount(); j++ {
				if gc := w.Child(j); gc != nil && gc.Type(lang) == t {
					set[[2]uint32{gc.StartByte(), gc.EndByte()}] = true
				}
			}
		}
	}
}

// boolOp returns the logical operator a node represents ("&&" / "||"), or ""
// when it isn't a logical-operator node.
func boolOp(n *ts.Node, lang *ts.Language) string {
	switch n.Type(lang) {
	case "conjunction_expression":
		return "&&"
	case "disjunction_expression":
		return "||"
	}
	for i := 0; i < n.ChildCount(); i++ {
		ch := n.Child(i)
		if ch == nil || ch.IsNamed() { // the operator is an anonymous token
			continue
		}
		switch ch.Type(lang) {
		case "&&", "and":
			return "&&"
		case "||", "or":
			return "||"
		}
	}
	return ""
}

// sameRunAsParent reports whether n's immediate parent is a logical-operator
// node with the same operator op — i.e. n continues an existing run.
func sameRunAsParent(n *ts.Node, op string, lang *ts.Language) bool {
	parent := n.Parent()
	return parent != nil && boolOp(parent, lang) == op
}
