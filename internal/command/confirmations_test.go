package command

import (
	"strings"
	"testing"
)

func TestParseConfirmations(t *testing.T) {
	body := `# Plan

## Confirmations

1. Which database should the new table live in? (default: PostgreSQL)
2. Should the migration run automatically on deploy? (default: no)
3. What feature flag name? (default: NEW_FEATURE)

## Implementation steps

- do something
`
	got := parseConfirmations(body)
	if len(got) != 3 {
		t.Fatalf("got %d confirmations, want 3: %+v", len(got), got)
	}
	want := []Confirmation{
		{Number: 1, Question: "Which database should the new table live in?", Default: "PostgreSQL"},
		{Number: 2, Question: "Should the migration run automatically on deploy?", Default: "no"},
		{Number: 3, Question: "What feature flag name?", Default: "NEW_FEATURE"},
	}
	for i, c := range want {
		if got[i] != c {
			t.Errorf("confirmation %d = %+v, want %+v", i, got[i], c)
		}
	}
}

func TestParseConfirmationsNoDefault(t *testing.T) {
	body := `## Confirmations

1. Which database should we use?
2. Keep the legacy endpoint? (default: no)
`
	got := parseConfirmations(body)
	if len(got) != 2 {
		t.Fatalf("got %d confirmations, want 2", len(got))
	}
	if got[0].Default != "" || got[0].Question != "Which database should we use?" {
		t.Errorf("confirmation 0 = %+v", got[0])
	}
	if got[1].Default != "no" {
		t.Errorf("confirmation 1 default = %q, want no", got[1].Default)
	}
}

func TestParseConfirmationsNone(t *testing.T) {
	for _, body := range []string{
		"## Confirmations\n\nNone.\n",
		"# Plan\n\n- no confirmations here\n",
	} {
		if got := parseConfirmations(body); len(got) != 0 {
			t.Errorf("parseConfirmations(%q) = %+v, want empty", body, got)
		}
	}
}

func TestParseDecisions(t *testing.T) {
	body := `## Decisions

- 1. PostgreSQL
- 2. yes
`
	got := parseDecisions(body)
	if len(got) != 2 {
		t.Fatalf("got %d decisions, want 2", len(got))
	}
	if got[1] != "PostgreSQL" || got[2] != "yes" {
		t.Errorf("decisions = %+v", got)
	}
}

func TestUnanswered(t *testing.T) {
	confirmations := []Confirmation{
		{Number: 1, Question: "a"},
		{Number: 2, Question: "b"},
		{Number: 3, Question: "c"},
	}
	decisions := map[int]string{1: "x"}
	got := unanswered(confirmations, decisions)
	if len(got) != 2 || got[0].Number != 2 || got[1].Number != 3 {
		t.Errorf("unanswered = %+v, want numbers 2,3", got)
	}
}

func TestWriteDecisionsAppends(t *testing.T) {
	body := `# Plan

## Confirmations

1. Which database? (default: PostgreSQL)

## Implementation steps

- do it
`
	out := writeDecisions(body, []Decision{{Number: 1, Answer: "postgres"}})
	if !strings.Contains(out, "## Decisions\n\n- 1. postgres\n") {
		t.Errorf("missing decisions section:\n%s", out)
	}
	if !strings.Contains(out, "## Implementation steps") {
		t.Errorf("implementation steps section lost:\n%s", out)
	}
	// a second write with a new answer must replace, not duplicate
	out = writeDecisions(out, []Decision{{Number: 2, Answer: "yes"}})
	if got := parseDecisions(out); got[1] != "postgres" || got[2] != "yes" || len(got) != 2 {
		t.Errorf("decisions after merge = %+v", got)
	}
	if strings.Count(out, "## Decisions") != 1 {
		t.Errorf("decisions section duplicated:\n%s", out)
	}
}

func TestWriteDecisionsReplaces(t *testing.T) {
	body := `# Plan

## Decisions

- 1. mysql

## Implementation steps

- do it
`
	out := writeDecisions(body, []Decision{{Number: 1, Answer: "postgres"}})
	if got := parseDecisions(out); got[1] != "postgres" || len(got) != 1 {
		t.Errorf("decisions = %+v, want postgres", got)
	}
	if strings.Count(out, "## Decisions") != 1 {
		t.Errorf("decisions section duplicated:\n%s", out)
	}
}

func TestPromptForAnswers(t *testing.T) {
	confirmations := []Confirmation{
		{Number: 1, Question: "Which database?", Default: "PostgreSQL"},
		{Number: 2, Question: "Run migration?", Default: "no"},
	}
	var buf strings.Builder
	got, err := promptForAnswers(confirmations, strings.NewReader("postgres\n\n"), &buf)
	if err != nil {
		t.Fatalf("promptForAnswers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d decisions, want 2", len(got))
	}
	if got[0].Answer != "postgres" {
		t.Errorf("answer 1 = %q, want postgres", got[0].Answer)
	}
	if got[1].Answer != "no" {
		t.Errorf("answer 2 = %q, want no (default)", got[1].Answer)
	}
	prompts := buf.String()
	if !strings.Contains(prompts, "1. Which database? [PostgreSQL]: ") || !strings.Contains(prompts, "2. Run migration? [no]: ") {
		t.Errorf("unexpected prompts:\n%s", prompts)
	}
}

func TestPromptForAnswersEOF(t *testing.T) {
	confirmations := []Confirmation{{Number: 1, Question: "Which database?", Default: "PostgreSQL"}}
	_, err := promptForAnswers(confirmations, strings.NewReader(""), &strings.Builder{})
	if err == nil {
		t.Fatal("expected error on EOF")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("error = %v, want cancelled", err)
	}
}
