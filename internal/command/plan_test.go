package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xynapse/internal/storage"
)

// stubOpenCode puts a fake `opencode` on PATH that prints the contents of the
// file named by FAKE_OUTPUT_FILE (a JSON event stream for ExtractText).
func stubOpenCode(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ncat \"$FAKE_OUTPUT_FILE\"\n"
	path := filepath.Join(dir, "opencode")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestPlanChecksOutBranch(t *testing.T) {
	dir := initRepo(t, "")
	runGit(t, dir, "checkout", "-q", "-b", "feature-v5/PROJ-1")
	runGit(t, dir, "checkout", "-q", "main")

	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")

	outputFile := filepath.Join(t.TempDir(), "output.json")
	if err := os.WriteFile(outputFile, []byte(`{"type":"text","part":{"type":"text","text":"# plan\n\n- step one"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_OUTPUT_FILE", outputFile)
	stubOpenCode(t)

	cfg := testConfig(base)
	captureStdout(t, func() {
		if err := Plan(cfg, "PROJ-1", dir, "", "feature-v5/PROJ-1"); err != nil {
			t.Fatalf("Plan: %v", err)
		}
	})

	if branch := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feature-v5/PROJ-1" {
		t.Errorf("current branch = %q, want feature-v5/PROJ-1", branch)
	}
	if _, status, ok := storage.NewStorage(base).ReadPlan("PROJ", "PROJ-1"); !ok {
		t.Error("plan file should have been written")
	} else if status != storage.PlanStatusNotStarted {
		t.Errorf("plan status = %q, want %q", status, storage.PlanStatusNotStarted)
	}
}

func TestPlanMissingBranch(t *testing.T) {
	dir := initRepo(t, "")
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")

	outputFile := filepath.Join(t.TempDir(), "output.json")
	if err := os.WriteFile(outputFile, []byte(`{"type":"text","part":{"type":"text","text":"# plan"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_OUTPUT_FILE", outputFile)
	stubOpenCode(t)

	cfg := testConfig(base)
	err := Plan(cfg, "PROJ-1", dir, "", "does-not-exist")
	if err == nil {
		t.Fatal("expected error for missing branch")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error should mention the branch, got %v", err)
	}
}

func TestPlanWithoutBranchStaysPut(t *testing.T) {
	dir := initRepo(t, "")
	runGit(t, dir, "checkout", "-q", "-b", "feature-v5/PROJ-1")

	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")

	outputFile := filepath.Join(t.TempDir(), "output.json")
	if err := os.WriteFile(outputFile, []byte(`{"type":"text","part":{"type":"text","text":"# plan"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_OUTPUT_FILE", outputFile)
	stubOpenCode(t)

	cfg := testConfig(base)
	captureStdout(t, func() {
		if err := Plan(cfg, "PROJ-1", dir, "", ""); err != nil {
			t.Fatalf("Plan: %v", err)
		}
	})

	if branch := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feature-v5/PROJ-1" {
		t.Errorf("current branch changed to %q without --branch", branch)
	}
}
