package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

func i(n int) *int { return &n }

func TestComputeFindings(t *testing.T) {
	rows := []row{
		{File: "a.go", Function: "Big", Cyclomatic: 20, Cognitive: i(30), StartLine: 1, EndLine: 40},
		{File: "a.go", Function: "Small", Cyclomatic: 2, Cognitive: i(1), StartLine: 50, EndLine: 55},
		{File: "b.rs", Function: "NoCognitive", Cyclomatic: 12, Cognitive: nil, StartLine: 1, EndLine: 9},
	}

	// Only cognitive threshold: one finding (Big).
	got := computeFindings(rows, 15, 0)
	if len(got) != 1 || got[0].Rule != ruleCognitive || got[0].Function != "Big" {
		t.Fatalf("cognitive-only findings = %+v", got)
	}

	// Both thresholds: Big trips both rules; NoCognitive trips cyclomatic only
	// (nil cognitive never trips the cognitive rule).
	got = computeFindings(rows, 15, 10)
	if len(got) != 3 {
		t.Fatalf("want 3 findings, got %d: %+v", len(got), got)
	}
	var cycOnNil bool
	for _, f := range got {
		if f.Function == "NoCognitive" {
			if f.Rule != ruleCyclomatic {
				t.Errorf("NoCognitive should only trip cyclomatic, got %s", f.Rule)
			}
			cycOnNil = true
		}
	}
	if !cycOnNil {
		t.Error("expected a cyclomatic finding for NoCognitive")
	}

	// No thresholds: no findings.
	if got := computeFindings(rows, 0, 0); len(got) != 0 {
		t.Errorf("no thresholds should yield no findings, got %+v", got)
	}
}

func TestFindingKeyIgnoresLineAndValue(t *testing.T) {
	a := finding{Rule: ruleCognitive, File: "x.go", Function: "F", Value: 30, StartLine: 10}
	b := finding{Rule: ruleCognitive, File: "x.go", Function: "F", Value: 99, StartLine: 200}
	if a.key() != b.key() {
		t.Errorf("key should ignore value/line: %q vs %q", a.key(), b.key())
	}
	c := finding{Rule: ruleCyclomatic, File: "x.go", Function: "F"}
	if a.key() == c.key() {
		t.Error("different rule should produce a different key")
	}
}

func TestBaselineRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "base.json")
	findings := []finding{
		{Rule: ruleCognitive, File: "a.go", Function: "F", Value: 30, StartLine: 1},
		{Rule: ruleCyclomatic, File: "a.go", Function: "F", Value: 20, StartLine: 1},
		// duplicate identity of the first — must be deduped on write
		{Rule: ruleCognitive, File: "a.go", Function: "F", Value: 31, StartLine: 5},
	}
	if err := writeBaseline(path, findings); err != nil {
		t.Fatal(err)
	}
	set, err := loadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(set) != 2 {
		t.Fatalf("want 2 deduped entries, got %d", len(set))
	}
	for _, f := range findings {
		if _, ok := set[f.key()]; !ok {
			t.Errorf("baseline missing %q", f.key())
		}
	}
	// A brand-new finding is not suppressed.
	fresh := finding{Rule: ruleCognitive, File: "b.go", Function: "G"}
	if _, ok := set[fresh.key()]; ok {
		t.Error("unrelated finding should not be in the baseline set")
	}
}

func TestEmitSARIF(t *testing.T) {
	findings := []finding{
		{Rule: ruleCognitive, File: "a.go", Function: "F", Value: 30, Threshold: 15, StartLine: 3, EndLine: 40},
	}
	var buf bytes.Buffer
	if err := emitSARIF(&buf, findings, 15, 0); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Runs []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID string `json:"id"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID string `json:"ruleId"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("SARIF is not valid JSON: %v", err)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("want 1 run, got %d", len(doc.Runs))
	}
	run := doc.Runs[0]
	if run.Tool.Driver.Name != "codemetrics" {
		t.Errorf("driver name = %q", run.Tool.Driver.Name)
	}
	// Only the cognitive rule is declared (cyclomatic threshold was 0).
	if len(run.Tool.Driver.Rules) != 1 || run.Tool.Driver.Rules[0].ID != ruleCognitive {
		t.Errorf("rules = %+v", run.Tool.Driver.Rules)
	}
	if len(run.Results) != 1 || run.Results[0].RuleID != ruleCognitive {
		t.Errorf("results = %+v", run.Results)
	}
}
