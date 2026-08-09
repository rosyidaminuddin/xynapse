package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareCreatesBranch(t *testing.T) {
	dir := initRepo(t, "")
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	cfg := testConfig(base)

	out := captureStdout(t, func() {
		if err := Prepare(cfg, "PROJ-1", dir, "main", "", false); err != nil {
			t.Fatalf("Prepare: %v", err)
		}
	})
	if !strings.Contains(out, "Created branch feature-v5/PROJ-1") {
		t.Errorf("output missing branch creation:\n%s", out)
	}
	if branch := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feature-v5/PROJ-1" {
		t.Errorf("current branch = %q, want feature-v5/PROJ-1", branch)
	}
}

func TestPrepareCustomTemplate(t *testing.T) {
	dir := initRepo(t, "")
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	cfg := testConfig(base)

	if err := Prepare(cfg, "PROJ-1", dir, "main", "{Project}/{TicketKey}", false); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if branch := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); branch != "PROJ/PROJ-1" {
		t.Errorf("current branch = %q, want PROJ/PROJ-1", branch)
	}
}

func TestPrepareDirtyTree(t *testing.T) {
	dir := initRepo(t, "")
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(base)

	if err := Prepare(cfg, "PROJ-1", dir, "main", "", false); err == nil {
		t.Fatal("expected error for dirty working tree")
	}
	if err := Prepare(cfg, "PROJ-1", dir, "main", "", true); err != nil {
		t.Fatalf("Prepare with force should proceed: %v", err)
	}
}

func TestPrepareExistingBranchCheckout(t *testing.T) {
	dir := initRepo(t, "")
	runGit(t, dir, "checkout", "-q", "-b", "feature-v5/PROJ-1")
	runGit(t, dir, "checkout", "-q", "main")

	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	cfg := testConfig(base)

	out := captureStdout(t, func() {
		if err := Prepare(cfg, "PROJ-1", dir, "main", "", false); err != nil {
			t.Fatalf("Prepare: %v", err)
		}
	})
	if !strings.Contains(out, "Already on branch feature-v5/PROJ-1") {
		t.Errorf("expected idempotent checkout, got:\n%s", out)
	}
	if branch := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feature-v5/PROJ-1" {
		t.Errorf("current branch = %q", branch)
	}
}

func TestPrepareMissingBaseBranch(t *testing.T) {
	dir := initRepo(t, "")
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	cfg := testConfig(base)

	if err := Prepare(cfg, "PROJ-1", dir, "does-not-exist", "", false); err == nil {
		t.Fatal("expected error for missing base branch")
	}
}
