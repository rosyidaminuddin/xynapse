package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xynapse/internal/config"
	"xynapse/internal/models"
	"xynapse/internal/storage"
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

func writeTicketOfType(t *testing.T, base, key, typ string) {
	t.Helper()
	s := storage.NewStorage(base)
	if err := s.WriteTicket(&models.Ticket{
		Key: key, Project: "PROJ", Summary: "Ticket " + key, Status: "Open", Type: typ,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPreparePerTypeTemplate(t *testing.T) {
	dir := initRepo(t, "")
	base := t.TempDir()
	writeTicketOfType(t, base, "PROJ-1", "Bug")
	cfg := testConfig(base)
	cfg.Git.BranchTemplates = map[string]string{"Bug": "fix-v5/{Key}"}

	if err := Prepare(cfg, "PROJ-1", dir, "main", "", false); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if branch := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); branch != "fix-v5/PROJ-1" {
		t.Errorf("current branch = %q, want fix-v5/PROJ-1", branch)
	}
}

func TestPreparePerTypeFallback(t *testing.T) {
	dir := initRepo(t, "")
	base := t.TempDir()
	writeTicketOfType(t, base, "PROJ-1", "Story")
	cfg := testConfig(base)
	cfg.Git.BranchTemplates = map[string]string{"Bug": "fix-v5/{Key}"}

	if err := Prepare(cfg, "PROJ-1", dir, "main", "", false); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if branch := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feature-v5/PROJ-1" {
		t.Errorf("current branch = %q, want feature-v5/PROJ-1 (fallback)", branch)
	}
}

func TestPrepareFlagOverridesPerType(t *testing.T) {
	dir := initRepo(t, "")
	base := t.TempDir()
	writeTicketOfType(t, base, "PROJ-1", "Bug")
	cfg := testConfig(base)
	cfg.Git.BranchTemplates = map[string]string{"Bug": "fix-v5/{Key}"}

	if err := Prepare(cfg, "PROJ-1", dir, "main", "custom/{TicketKey}", false); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if branch := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); branch != "custom/PROJ-1" {
		t.Errorf("current branch = %q, want custom/PROJ-1", branch)
	}
}

func TestPrepareUsesResolvedProjectConfig(t *testing.T) {
	dir := initRepo(t, "")
	base := t.TempDir()
	writeTicketOfType(t, base, "PROJ-1", "Bug")
	cfg := &config.Config{
		Defaults: config.Defaults{Project: "OTHER"},
		Storage:  config.StorageConfig{Base: base},
		Git:      config.GitConfig{BranchTemplate: "feature-v5/{Key}"},
		Projects: map[string]config.ProjectConfig{
			"PROJ": {Git: config.GitConfig{
				BranchTemplate:  "release/{Key}",
				BranchTemplates: map[string]string{"Bug": "fix-v5/{Key}"},
			}},
		},
	}

	resolved, err := cfg.ResolveProject("PROJ")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if err := Prepare(resolved, "PROJ-1", dir, "main", "", false); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if branch := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); branch != "fix-v5/PROJ-1" {
		t.Errorf("current branch = %q, want fix-v5/PROJ-1 (per-project per-type template)", branch)
	}
}

func TestPrepareUnconfiguredProjectFallsBackToGlobal(t *testing.T) {
	dir := initRepo(t, "")
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	cfg := &config.Config{
		Storage: config.StorageConfig{Base: base},
		Git:     config.GitConfig{BranchTemplate: "feature-v5/{Key}"},
		Projects: map[string]config.ProjectConfig{
			"ALPHA": {Git: config.GitConfig{BranchTemplate: "release/{Key}"}},
		},
	}

	resolved, err := cfg.ResolveProject("PROJ")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if err := Prepare(resolved, "PROJ-1", dir, "main", "", false); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if branch := gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); branch != "feature-v5/PROJ-1" {
		t.Errorf("current branch = %q, want feature-v5/PROJ-1 (global fallback)", branch)
	}
}
