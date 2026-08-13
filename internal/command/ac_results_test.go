package command

import "testing"

func TestParseACResults(t *testing.T) {
	md := `# Implement report

Implemented all the things.

## AC Results

- [x] AC1: list is sorted
- [ ] AC2: empty input handled
- [x] AC3: errors surface
`
	results, ok := parseACResults(md)
	if !ok {
		t.Fatal("expected AC Results section found")
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	if !results[0].Pass || results[0].Text != "AC1: list is sorted" {
		t.Errorf("result 0 = %+v", results[0])
	}
	if results[1].Pass {
		t.Errorf("result 1 should be a fail")
	}
	if !results[2].Pass {
		t.Errorf("result 2 should be a pass")
	}
}

func TestParseACResultsUppercaseX(t *testing.T) {
	md := "## AC Results\n\n- [X] all good\n"
	results, ok := parseACResults(md)
	if !ok {
		t.Fatal("expected section")
	}
	if len(results) != 1 || !results[0].Pass {
		t.Errorf("results = %+v, want one pass", results)
	}
}

func TestParseACResultsAbsent(t *testing.T) {
	md := "# Report\n\nNo AC section here.\n"
	if results, ok := parseACResults(md); ok || results != nil {
		t.Errorf("parseACResults = %v, %v; want nil, false", results, !ok)
	}
}

func TestParseACResultsEmptySection(t *testing.T) {
	md := "## AC Results\n\n\n"
	results, ok := parseACResults(md)
	if !ok {
		t.Fatal("expected section found even if empty")
	}
	if len(results) != 0 {
		t.Errorf("results = %v, want empty", results)
	}
}

func TestACResultsSummary(t *testing.T) {
	cases := []struct {
		results []ACResult
		want    string
	}{
		{nil, "no acceptance criteria results reported"},
		{[]ACResult{{Pass: true}, {Pass: true}}, "all acceptance criteria met"},
		{[]ACResult{{Pass: false}, {Pass: false}}, "no acceptance criteria met"},
		{[]ACResult{{Pass: true}, {Pass: false}}, "1 met, 1 not met"},
	}
	for _, tc := range cases {
		if got := acResultsSummary(tc.results); got != tc.want {
			t.Errorf("acResultsSummary(%v) = %q, want %q", tc.results, got, tc.want)
		}
	}
}