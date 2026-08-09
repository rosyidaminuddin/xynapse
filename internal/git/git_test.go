package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

// initRepo creates a git repo in a temp dir with an initial commit on main.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "checkout", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func TestCurrentBranch(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")
	branch, err := g.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" {
		t.Errorf("CurrentBranch = %q, want main", branch)
	}
}

func TestBranchLifecycle(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")

	if !g.BranchExists("main") {
		t.Error("expected main branch to exist")
	}
	if g.BranchExists("feature-x") {
		t.Error("feature-x should not exist yet")
	}

	if err := g.CreateBranch("feature-x"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if !g.BranchExists("feature-x") {
		t.Error("feature-x should exist after creation")
	}
	branch, _ := g.CurrentBranch()
	if branch != "feature-x" {
		t.Errorf("CurrentBranch = %q, want feature-x", branch)
	}

	if err := g.Checkout("main"); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	branch, _ = g.CurrentBranch()
	if branch != "main" {
		t.Errorf("CurrentBranch = %q, want main", branch)
	}
}

func TestIsDirtyAndCommit(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")

	dirty, err := g.IsDirty()
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Error("expected clean working tree after init commit")
	}

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, _ = g.IsDirty()
	if !dirty {
		t.Error("expected dirty working tree after modification")
	}
	status, _ := g.StatusPorcelain()
	if !strings.Contains(status, "M file.txt") {
		t.Errorf("StatusPorcelain should show modified file.txt, got %q", status)
	}

	if err := g.AddAll(); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	if err := g.Commit("update file.txt"); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	dirty, _ = g.IsDirty()
	if dirty {
		t.Error("expected clean working tree after commit")
	}
}

func TestCreateBranchFromBase(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")
	if err := g.Checkout("main"); err != nil {
		t.Fatal(err)
	}
	if err := g.CreateBranch("feature-v5/PROJ-1"); err != nil {
		t.Fatalf("CreateBranch with slash: %v", err)
	}
	branch, _ := g.CurrentBranch()
	if branch != "feature-v5/PROJ-1" {
		t.Errorf("CurrentBranch = %q", branch)
	}
}
