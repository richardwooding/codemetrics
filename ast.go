package codemetrics

import "go/ast"

// Cyclomatic returns the McCabe cyclomatic complexity of a function body —
// 1 + one per branch point (if / for / range / case / comm-clause / && / ||).
//
// Use this when you already hold a parsed AST (e.g. from your own go/parser
// pass); [ParseGo] derives the same value from source. A nil body returns 0.
func Cyclomatic(body *ast.BlockStmt) int {
	if body == nil {
		return 0
	}
	return goComplexity(body)
}

// Cognitive returns the SonarSource cognitive complexity of a Go function
// declaration. Use this when you already hold a parsed AST; [ParseGo] derives
// the same value from source. A nil declaration (or one without a name)
// returns 0.
//
// See the package documentation for the increment rules.
func Cognitive(fn *ast.FuncDecl) int {
	if fn == nil || fn.Name == nil {
		return 0
	}
	return goCognitiveComplexity(fn)
}
