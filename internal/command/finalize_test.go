package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xynapse/internal/storage"
)

func TestFinalizeCommitsAndPushes(t *testing.T) {
	remote := initRemote(t)
	dir := initRepo(t, remote)
	runGit(t, dir, "checkout", "-q", "-b", "feature-v5/PROJ-1")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("implemented\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	writePlanFile(t, base, "PROJ-1", []byte("# plan"))
	cfg := testConfig(base)

	out := captureStdout(t, func() {
		if err := Finalize(cfg, "PROJ-1", dir, "", "", false); err != nil {
			t.Fatalf("Finalize: %v", err)
		}
	})
	for _, want := range []string{"Committed", "Pushed feature-v5/PROJ-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}

	if status, ok := storage.NewStorage(base).PlanStatus("PROJ", "PROJ-1"); !ok || status != storage.PlanStatusDone {
		t.Errorf("plan status = %q (ok=%v), want done", status, ok)
	}

	runGit(t, dir, "fetch", "-q", "origin")
	if msg := gitOut(t, dir, "log", "-1", "--format=%s", "origin/feature-v5/PROJ-1"); !strings.Contains(msg, "PROJ-1: Add stuff") {
		t.Errorf("remote commit message = %q, want PROJ-1: Add stuff", msg)
	}
}

func TestFinalizeCleanTreeStillPushes(t *testing.T) {
	remote := initRemote(t)
	dir := initRepo(t, remote)
	runGit(t, dir, "checkout", "-q", "-b", "feature-v5/PROJ-1")

	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	writePlanFile(t, base, "PROJ-1", []byte("# plan"))
	cfg := testConfig(base)

	out := captureStdout(t, func() {
		if err := Finalize(cfg, "PROJ-1", dir, "", "", false); err != nil {
			t.Fatalf("Finalize: %v", err)
		}
	})
	if !strings.Contains(out, "No changes to commit") {
		t.Errorf("expected clean-tree notice:\n%s", out)
	}
	if !strings.Contains(out, "Pushed feature-v5/PROJ-1") {
		t.Errorf("expected push notice:\n%s", out)
	}
}

func TestFinalizeRefusesOnBaseBranch(t *testing.T) {
	remote := initRemote(t)
	dir := initRepo(t, remote)
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	cfg := testConfig(base)

	if err := Finalize(cfg, "PROJ-1", dir, "main", "", false); err == nil {
		t.Fatal("expected error when finalizing on the base branch")
	}
}

func TestFinalizeCreatesPR(t *testing.T) {
	remote := initRemote(t)
	dir := initRepo(t, remote)
	runGit(t, dir, "checkout", "-q", "-b", "feature-v5/PROJ-1")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("implemented\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	argsFile := filepath.Join(t.TempDir(), "gh_args.txt")
	stubGH(t, argsFile)

	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	writePlanFile(t, base, "PROJ-1", []byte("# plan"))
	cfg := testConfig(base)
	cfg.Jira.URL = "https://example.atlassian.net"

	out := captureStdout(t, func() {
		if err := Finalize(cfg, "PROJ-1", dir, "main", "", true); err != nil {
			t.Fatalf("Finalize: %v", err)
		}
	})
	if !strings.Contains(out, "Pull request created: https://github.com/example/xynapse/pull/1") {
		t.Errorf("output missing PR URL:\n%s", out)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--base", "main", "--head", "feature-v5/PROJ-1", "--title", "PROJ-1: Add stuff", "--body", "Closes PROJ-1"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("gh args missing %q: %q", want, string(data))
		}
	}
}

func TestFinalizePRWithoutBase(t *testing.T) {
	remote := initRemote(t)
	dir := initRepo(t, remote)
	runGit(t, dir, "checkout", "-q", "-b", "feature-v5/PROJ-1")

	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	writePlanFile(t, base, "PROJ-1", []byte("# plan"))
	cfg := testConfig(base)

	if err := Finalize(cfg, "PROJ-1", dir, "", "", true); err == nil {
		t.Fatal("expected error when --pr is used without a base branch")
	}
}
