package treesitter_test

import (
	"errors"
	"testing"

	codemetrics "github.com/richardwooding/codemetrics"
	"github.com/richardwooding/codemetrics/treesitter"
)

// cognitiveByFunc maps function name -> cognitive complexity for the given
// source. Functions whose language has no cognitive spec are omitted.
func cognitiveByFunc(t *testing.T, language, src string) map[string]int {
	t.Helper()
	fns, err := treesitter.Parse(language, []byte(src))
	if err != nil {
		t.Fatalf("Parse(%s): %v", language, err)
	}
	out := map[string]int{}
	for _, f := range fns {
		if f.Cognitive != nil {
			out[f.Name] = *f.Cognitive
		}
	}
	return out
}

func TestCognitiveComplexity(t *testing.T) {
	cases := []struct {
		name, language, fn, src string
		want                    int
	}{
		{
			name: "python-nested", language: "python", fn: "branchy",
			src: "def branchy(x):\n" +
				"    if x > 0:\n" +
				"        for i in range(x):\n" +
				"            if i % 2 == 0:\n" +
				"                return i\n" +
				"    return 0\n",
			want: 6,
		},
		{
			name: "python-elif", language: "python", fn: "f",
			src: "def f(n):\n" +
				"    if n == 1:\n" +
				"        return 1\n" +
				"    elif n == 2:\n" +
				"        return 2\n" +
				"    else:\n" +
				"        return 3\n",
			want: 3,
		},
		{
			name: "python-flat", language: "python", fn: "f",
			src: "def f(a, b):\n" +
				"    if a:\n" +
				"        return 1\n" +
				"    if b:\n" +
				"        return 2\n" +
				"    return 0\n",
			want: 2,
		},
		{
			name: "python-bool", language: "python", fn: "f",
			src: "def f(a, b, c):\n" +
				"    if a and b and c:\n" +
				"        return 1\n" +
				"    return 0\n",
			want: 2,
		},
		{
			name: "js-nested", language: "javascript", fn: "branchy",
			src: "function branchy(x) {\n" +
				"  if (x > 0) {\n" +
				"    for (let i = 0; i < x; i++) {\n" +
				"      if (i % 2 === 0) { return i; }\n" +
				"    }\n" +
				"  }\n" +
				"  return 0;\n" +
				"}\n",
			want: 6,
		},
		{
			name: "js-elseif", language: "javascript", fn: "f",
			src: "function f(n) {\n" +
				"  if (n === 1) { return 1; }\n" +
				"  else if (n === 2) { return 2; }\n" +
				"  else { return 3; }\n" +
				"}\n",
			want: 3,
		},
		{
			name: "ts-switch", language: "typescript", fn: "f",
			src: "function f(n: number): string {\n" +
				"  switch (n) {\n" +
				"    case 1: return \"one\";\n" +
				"    case 2: return \"two\";\n" +
				"    default: return \"lots\";\n" +
				"  }\n" +
				"}\n",
			want: 1,
		},
		{
			name: "java-nested", language: "java", fn: "branchy",
			src: "class C {\n" +
				"  int branchy(int x) {\n" +
				"    if (x > 0) {\n" +
				"      for (int i = 0; i < x; i++) {\n" +
				"        if (i % 2 == 0) { return i; }\n" +
				"      }\n" +
				"    }\n" +
				"    return 0;\n" +
				"  }\n" +
				"}\n",
			want: 6,
		},
		{
			name: "java-switch", language: "java", fn: "f",
			src: "class C {\n" +
				"  String f(int n) {\n" +
				"    switch (n) {\n" +
				"      case 1: return \"one\";\n" +
				"      default: return \"lots\";\n" +
				"    }\n" +
				"  }\n" +
				"}\n",
			want: 1,
		},
		{
			name: "js-forof", language: "javascript", fn: "f",
			src: "function f(xs) {\n" +
				"  for (const x of xs) {\n" +
				"    if (x > 0) { return x; }\n" +
				"  }\n" +
				"  return 0;\n" +
				"}\n",
			want: 3,
		},
		{
			name: "rust-nested", language: "rust", fn: "branchy",
			src: "fn branchy(x: i32) -> i32 {\n" +
				"    if x > 0 {\n" +
				"        for i in 0..x {\n" +
				"            if i % 2 == 0 {\n" +
				"                return i;\n" +
				"            }\n" +
				"        }\n" +
				"    }\n" +
				"    return 0;\n" +
				"}\n",
			want: 6,
		},
		{
			name: "c-nested", language: "c", fn: "branchy",
			src:  "int branchy(int x){\n  if(x>0){\n    for(int i=0;i<x;i++){\n      if(i%2==0){ return i; }\n    }\n  }\n  return 0;\n}\n",
			want: 6,
		},
		{
			name: "c-elseif", language: "c", fn: "f",
			src:  "int f(int n){\n  if(n==1){ return 1; }\n  else if(n==2){ return 2; }\n  return 0;\n}\n",
			want: 2,
		},
		{
			name: "c-switch", language: "c", fn: "f",
			src:  "int f(int n){\n  switch(n){\n    case 1: return 1;\n    default: return 0;\n  }\n}\n",
			want: 1,
		},
		{
			name: "cpp-trycatch", language: "cpp", fn: "f",
			src:  "int f(int x){\n  if(x>0){\n    try { g(); } catch(int e){ if(e>0){ return e; } }\n  }\n  return 0;\n}\n",
			want: 6,
		},
		{
			name: "csharp-nested", language: "csharp", fn: "Branchy",
			src:  "class C{\n  int Branchy(int x){\n    if(x>0){\n      for(int i=0;i<x;i++){\n        if(i%2==0){ return i; }\n      }\n    }\n    return 0;\n  }\n}\n",
			want: 6,
		},
		{
			name: "csharp-elseif", language: "csharp", fn: "F",
			src:  "class C{\n  int F(int n){\n    if(n==1){ return 1; }\n    else if(n==2){ return 2; }\n    return 0;\n  }\n}\n",
			want: 2,
		},
		{
			name: "csharp-elseif-chain", language: "csharp", fn: "F",
			src:  "class C{\n  int F(int n){\n    if(n==1){ return 1; }\n    else if(n==2){ return 2; }\n    else if(n==3){ return 3; }\n    else if(n==4){ return 4; }\n    return 0;\n  }\n}\n",
			want: 4,
		},
		{
			name: "python-elif-chain", language: "python", fn: "f",
			src: "def f(n):\n" +
				"    if n == 1:\n        return 1\n" +
				"    elif n == 2:\n        return 2\n" +
				"    elif n == 3:\n        return 3\n" +
				"    elif n == 4:\n        return 4\n" +
				"    return 0\n",
			want: 4,
		},
		{
			name: "kotlin-nested", language: "kotlin", fn: "branchy",
			src:  "fun branchy(x: Int): Int {\n  if (x > 0) {\n    for (i in 0..x) {\n      if (i % 2 == 0) { return i }\n    }\n  }\n  return 0\n}\n",
			want: 6,
		},
		{
			name: "kotlin-when", language: "kotlin", fn: "f",
			src:  "fun f(n: Int): Int {\n  return when (n) {\n    1 -> 1\n    2 -> 2\n    else -> 0\n  }\n}\n",
			want: 1,
		},
		{
			name: "php-nested", language: "php", fn: "branchy",
			src:  "<?php function branchy($x){\n  if($x>0){\n    foreach($xs as $i){\n      if($i%2==0){ return $i; }\n    }\n  }\n  return 0;\n}\n",
			want: 6,
		},
		{
			name: "php-elseif", language: "php", fn: "f",
			src:  "<?php function f($n){\n  if($n==1){ return 1; }\n  elseif($n==2){ return 2; }\n  else { return 3; }\n  return 0;\n}\n",
			want: 3,
		},
		{
			name: "ruby-nested", language: "ruby", fn: "branchy",
			src:  "def branchy(x)\n  if x > 0\n    while x > 0\n      if x > 5\n        x -= 1\n      end\n    end\n  end\n  0\nend\n",
			want: 6,
		},
		{
			name: "ruby-case", language: "ruby", fn: "f",
			src:  "def f(x)\n  case x\n  when 1 then 1\n  else 0\n  end\nend\n",
			want: 1,
		},
		{
			name: "ruby-elsif", language: "ruby", fn: "f",
			src:  "def f(n)\n  if n == 1\n    1\n  elsif n == 2\n    2\n  end\nend\n",
			want: 2,
		},
		{
			name: "scala-nested", language: "scala", fn: "branchy",
			src:  "def branchy(x: Int): Int = {\n  if (x > 0) {\n    for (i <- 0 until x) {\n      if (i % 2 == 0) return i\n    }\n  }\n  0\n}\n",
			want: 6,
		},
		{
			name: "scala-match", language: "scala", fn: "f",
			src:  "def f(x: Int): Int = {\n  x match {\n    case 1 => 1\n    case _ => 0\n  }\n}\n",
			want: 1,
		},
		{
			name: "r-nested", language: "r", fn: "branchy",
			src:  "branchy <- function(x) {\n  if (x > 0) {\n    for (i in 1:x) {\n      if (i %% 2 == 0) return(i)\n    }\n  }\n  0\n}\n",
			want: 6,
		},
		{
			name: "matlab-nested", language: "matlab", fn: "branchy",
			src:  "function r = branchy(x)\n  r = 0;\n  if x > 0\n    for i = 1:x\n      if mod(i,2) == 0\n        r = i;\n      end\n    end\n  end\nend\n",
			want: 6,
		},
		{
			name: "matlab-elseif", language: "matlab", fn: "f",
			src:  "function r = f(n)\n  if n == 1\n    r = 1;\n  elseif n == 2\n    r = 2;\n  else\n    r = 3;\n  end\nend\n",
			want: 3,
		},
		{
			name: "perl-nested", language: "perl", fn: "branchy",
			src:  "sub branchy {\n  my $x = shift;\n  if ($x > 0) {\n    while ($x > 0) {\n      if ($x > 5) { $x--; }\n    }\n  }\n  return 0;\n}\n",
			want: 6,
		},
		{
			name: "perl-elsif", language: "perl", fn: "f",
			src:  "sub f {\n  my $x = shift;\n  if ($x == 1) { return 1; }\n  elsif ($x == 2) { return 2; }\n  else { return 3; }\n}\n",
			want: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cognitiveByFunc(t, tc.language, tc.src)
			v, ok := got[tc.fn]
			if !ok {
				t.Fatalf("no cognitive value for %q; got %+v", tc.fn, got)
			}
			if v != tc.want {
				t.Errorf("cognitive(%s) = %d, want %d", tc.fn, v, tc.want)
			}
		})
	}
}

func TestCyclomaticAndSpans(t *testing.T) {
	// A function with two ifs and an && → cyclomatic 1 + 2 + 1 = 4.
	src := "def f(a, b, c):\n" +
		"    if a and b:\n" +
		"        return 1\n" +
		"    if c:\n" +
		"        return 2\n" +
		"    return 0\n"
	fns, err := treesitter.Parse("python", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var f *codemetrics.FunctionMetrics
	for i := range fns {
		if fns[i].Name == "f" {
			f = &fns[i]
		}
	}
	if f == nil {
		t.Fatalf("function f not found in %+v", fns)
	}
	if f.Cyclomatic != 4 {
		t.Errorf("cyclomatic = %d, want 4", f.Cyclomatic)
	}
	if f.StartLine != 1 {
		t.Errorf("StartLine = %d, want 1", f.StartLine)
	}
	if f.EndLine < f.StartLine {
		t.Errorf("EndLine %d < StartLine %d", f.EndLine, f.StartLine)
	}
	if f.Cognitive == nil {
		t.Error("Cognitive is nil for Python; want a value")
	}
}

func TestSwiftCognitiveAvailable(t *testing.T) {
	// Swift gained a cognitive spec once gotreesitter v0.20.7 fixed the else-if
	// mis-parse (#131 / file-search-on#491): a single if reports Cognitive 1,
	// and Cyclomatic is still reported.
	src := "func f(_ x: Int) -> Int {\n  if x > 0 { return 1 }\n  return 0\n}\n"
	fns, err := treesitter.Parse("swift", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(fns) == 0 {
		t.Fatal("no functions parsed for swift")
	}
	for _, f := range fns {
		if f.Cognitive == nil {
			t.Errorf("swift %s Cognitive = nil, want a value", f.Name)
			continue
		}
		if *f.Cognitive != 1 {
			t.Errorf("swift %s Cognitive = %d, want 1", f.Name, *f.Cognitive)
		}
		if f.Cyclomatic < 1 {
			t.Errorf("swift %s Cyclomatic = %d, want >= 1", f.Name, f.Cyclomatic)
		}
	}
}

// TestSwiftCognitiveElseIf locks in the else-if regression: an else-if chain
// must charge the flat continuation cost (not a nested-if penalty). For
// `if {…} else if {…} else if {…}` the cognitive score is 1 + 1 + 1 = 3.
func TestSwiftCognitiveElseIf(t *testing.T) {
	src := "func f(_ x: Int) -> Int {\n" +
		"  if x > 0 { return 1 } else if x < 0 { return 2 } else if x == 0 { return 3 }\n" +
		"  return 4\n}\n"
	got := cognitiveByFunc(t, "swift", src)
	if got["f"] != 3 {
		t.Errorf("swift else-if cognitive = %d, want 3 (%v)", got["f"], got)
	}
}

func TestUnsupportedLanguage(t *testing.T) {
	_, err := treesitter.Parse("go", []byte("package p"))
	if !errors.Is(err, codemetrics.ErrUnsupportedLanguage) {
		t.Errorf("Parse(go) error = %v, want ErrUnsupportedLanguage (use the root package for Go)", err)
	}
}

func TestSupportedLanguages(t *testing.T) {
	got := treesitter.SupportedLanguages()
	if len(got) != 16 {
		t.Errorf("SupportedLanguages() has %d entries, want 16: %v", len(got), got)
	}
	// sorted + contains a couple of known ones
	want := map[string]bool{"python": true, "rust": true, "scala": true}
	for _, l := range got {
		delete(want, l)
	}
	if len(want) != 0 {
		t.Errorf("missing languages: %v", want)
	}
}
