package codemetrics

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// parseFirstFuncDecl returns the first *ast.FuncDecl in src.
func parseFirstFuncDecl(t *testing.T, src string) *ast.FuncDecl {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			return fn
		}
	}
	t.Fatal("no function declaration in source")
	return nil
}

// The AST-level helpers must agree with the source-level ParseGo path.
func TestASTHelpersMatchParseGo(t *testing.T) {
	src := `package p
func classify(n int) string {
	if n < 0 {
		return "neg"
	} else if n == 0 {
		return "zero"
	}
	for i := 0; i < n; i++ {
		if i%2 == 0 && i > 0 {
			println(i)
		}
	}
	return "pos"
}`
	fns, err := ParseGo([]byte(src))
	if err != nil {
		t.Fatalf("ParseGo: %v", err)
	}
	if len(fns) != 1 {
		t.Fatalf("got %d functions, want 1", len(fns))
	}
	want := fns[0]

	fn := parseFirstFuncDecl(t, src)
	if got := Cyclomatic(fn.Body); got != want.Cyclomatic {
		t.Errorf("Cyclomatic = %d, want %d (ParseGo)", got, want.Cyclomatic)
	}
	if got := Cognitive(fn); got != *want.Cognitive {
		t.Errorf("Cognitive = %d, want %d (ParseGo)", got, *want.Cognitive)
	}
}

func TestASTHelpersNilSafe(t *testing.T) {
	if got := Cyclomatic(nil); got != 0 {
		t.Errorf("Cyclomatic(nil) = %d, want 0", got)
	}
	if got := Cognitive(nil); got != 0 {
		t.Errorf("Cognitive(nil) = %d, want 0", got)
	}
	if got := Cognitive(&ast.FuncDecl{}); got != 0 {
		t.Errorf("Cognitive(nameless) = %d, want 0", got)
	}
}
