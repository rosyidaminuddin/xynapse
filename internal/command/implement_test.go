package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xynapse/internal/storage"
)

// stubOpenCodeArgs puts a fake `opencode` on PATH that writes its arguments to
// the file named by OPENCODE_ARGS_FILE and then prints the JSON event stream
// from FAKE_OUTPUT_FILE.
func stubOpenCodeArgs(t *testing.T, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$OPENCODE_ARGS_FILE\"\ncat \"$FAKE_OUTPUT_FILE\"\n"
	path := filepath.Join(dir, "opencode")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCODE_ARGS_FILE", argsFile)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func fakeOutputFile(t *testing.T, text string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "output.json")
	if err := os.WriteFile(f, []byte(`{"type":"text","part":{"type":"text","text":"`+text+`"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_OUTPUT_FILE", f)
	return f
}

const confirmationPlan = `---
status: not started
---

# Plan

## Confirmations

1. Which database should we use? (default: PostgreSQL)
2. Should the migration run automatically? (default: no)

## Implementation steps

- implement something
`

func TestImplementPromptsForConfirmations(t *testing.T) {
	dir := t.TempDir()
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	writePlanFile(t, base, "PROJ-1", []byte(confirmationPlan))

	argsFile := filepath.Join(t.TempDir(), "args.txt")
	fakeOutputFile(t, "# report")
	stubOpenCodeArgs(t, argsFile)

	prev := confirmReader
	confirmReader = strings.NewReader("postgres\nyes\n")
	t.Cleanup(func() { confirmReader = prev })

	cfg := testConfig(base)
	captureStdout(t, func() {
		if err := Implement(cfg, "PROJ-1", "", dir, "", false); err != nil {
			t.Fatalf("Implement: %v", err)
		}
	})

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("opencode was not invoked: %v", err)
	}
	prompt := strings.Join(strings.Fields(string(args)), " ")
	if !strings.Contains(prompt, "- 1. postgres") || !strings.Contains(prompt, "- 2. yes") {
		t.Errorf("opencode prompt missing decisions:\n%s", args)
	}

	_, status, ok := storage.NewStorage(base).ReadPlan("PROJ", "PROJ-1")
	if !ok {
		t.Fatal("plan file missing")
	}
	if status != storage.PlanStatusInProgress {
		t.Errorf("plan status = %q, want %q", status, storage.PlanStatusInProgress)
	}
	body, _, _ := storage.NewStorage(base).ReadPlan("PROJ", "PROJ-1")
	if !strings.Contains(body, "- 1. postgres") || !strings.Contains(body, "- 2. yes") {
		t.Errorf("decisions not persisted in plan:\n%s", body)
	}
}

func TestImplementAbortsOnCancel(t *testing.T) {
	dir := t.TempDir()
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	writePlanFile(t, base, "PROJ-1", []byte(confirmationPlan))

	argsFile := filepath.Join(t.TempDir(), "args.txt")
	fakeOutputFile(t, "# report")
	stubOpenCodeArgs(t, argsFile)

	prev := confirmReader
	confirmReader = strings.NewReader("")
	t.Cleanup(func() { confirmReader = prev })

	cfg := testConfig(base)
	err := Implement(cfg, "PROJ-1", "", dir, "", false)
	if err == nil {
		t.Fatal("expected error when confirmations are not answered")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("error = %v, want cancelled", err)
	}
	if _, err := os.Stat(argsFile); !os.IsNotExist(err) {
		t.Errorf("opencode should not have run (args file present)")
	}
}

func TestImplementSkipsPromptWhenAnswered(t *testing.T) {
	dir := t.TempDir()
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	plan := confirmationPlan + "\n## Decisions\n\n- 1. postgres\n- 2. yes\n"
	writePlanFile(t, base, "PROJ-1", []byte(plan))

	argsFile := filepath.Join(t.TempDir(), "args.txt")
	fakeOutputFile(t, "# report")
	stubOpenCodeArgs(t, argsFile)

	cfg := testConfig(base)
	captureStdout(t, func() {
		if err := Implement(cfg, "PROJ-1", "", dir, "", false); err != nil {
			t.Fatalf("Implement: %v", err)
		}
	})

	if _, err := os.Stat(argsFile); err != nil {
		t.Fatalf("opencode should have run: %v", err)
	}
	body, _, _ := storage.NewStorage(base).ReadPlan("PROJ", "PROJ-1")
	if !strings.Contains(body, "- 1. postgres") {
		t.Errorf("existing decisions lost:\n%s", body)
	}
}

func TestImplementNoConfirmationsNoPrompt(t *testing.T) {
	dir := t.TempDir()
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	writePlanFile(t, base, "PROJ-1", []byte("# Plan\n\n- do something\n"))

	argsFile := filepath.Join(t.TempDir(), "args.txt")
	fakeOutputFile(t, "# report")
	stubOpenCodeArgs(t, argsFile)

	cfg := testConfig(base)
	captureStdout(t, func() {
		if err := Implement(cfg, "PROJ-1", "", dir, "", false); err != nil {
			t.Fatalf("Implement: %v", err)
		}
	})

	if _, err := os.Stat(argsFile); err != nil {
		t.Fatalf("opencode should have run: %v", err)
	}
}
