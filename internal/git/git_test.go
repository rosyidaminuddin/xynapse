package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestIsInsideWorkTree(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")
	ok, err := g.IsInsideWorkTree()
	if err != nil {
		t.Fatalf("IsInsideWorkTree: %v", err)
	}
	if !ok {
		t.Error("expected true inside a repo")
	}

	outside := New(t.TempDir(), "")
	if ok, _ := outside.IsInsideWorkTree(); ok {
		t.Error("expected false outside a repo")
	}
}

func TestLocalBranches(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")
	if err := g.CreateBranch("feature-v5/PROJ-1"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	branches, err := g.LocalBranches()
	if err != nil {
		t.Fatalf("LocalBranches: %v", err)
	}
	got := strings.Join(branches, ",")
	for _, want := range []string{"main", "feature-v5/PROJ-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("LocalBranches missing %q: %q", want, got)
		}
	}
}

func TestCommitSubjects(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")
	if err := g.CreateBranch("feature-v5/PROJ-1"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "proj.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "PROJ-1: Add stuff")

	subjects, err := g.CommitSubjects()
	if err != nil {
		t.Fatalf("CommitSubjects: %v", err)
	}
	if !contains(subjects, "PROJ-1: Add stuff") {
		t.Errorf("CommitSubjects = %v, want PROJ-1: Add stuff", subjects)
	}
}

func TestCommitSubjectsEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	g := New(dir, "")
	subjects, err := g.CommitSubjects()
	if err != nil {
		t.Fatalf("CommitSubjects on empty repo: %v", err)
	}
	if len(subjects) != 0 {
		t.Errorf("expected no subjects, got %v", subjects)
	}
}

func TestTestCommand(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")

	out, err := g.Test("echo hello && echo world")
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if out != "hello\nworld" {
		t.Errorf("Test output = %q", out)
	}
}

func TestTestCommandFailure(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")

	out, err := g.Test("echo boom; exit 3")
	if err == nil {
		t.Fatal("expected error for failing command")
	}
	if !strings.Contains(out, "boom") {
		t.Errorf("Test output should include command output, got %q", out)
	}
	if !strings.Contains(err.Error(), "exit status 3") {
		t.Errorf("error should mention exit code, got %v", err)
	}
}

func TestTestCommandEmpty(t *testing.T) {
	g := New(initRepo(t), "")
	if _, err := g.Test("   "); err == nil {
		t.Error("expected error for empty command")
	}
}

func TestTestCommandRunsInRepoDir(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")
	out, err := g.Test("git rev-parse --abbrev-ref HEAD")
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if out != "main" {
		t.Errorf("git command should run inside the repo dir, got %q", out)
	}
}

func TestTestCommandFiltersSecretsFromChildEnv(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")
	t.Setenv("JIRA_API_TOKEN", "super-secret")
	t.Setenv("JIRA_EMAIL", "me@corp.com")

	out, err := g.Test("env | sort")
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if strings.Contains(out, "JIRA_API_TOKEN") {
		t.Errorf("JIRA_API_TOKEN leaked into child env:\n%s", out)
	}
	if !strings.Contains(out, "JIRA_EMAIL") {
		t.Errorf("JIRA_EMAIL should be kept in child env:\n%s", out)
	}
}

func TestTestCommandTimeoutKillsHangingCommand(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")
	start := time.Now()
	_, err := g.TestContext("sleep 30", 300*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("command should have been killed by timeout, took %v", elapsed)
	}
	if !strings.Contains(err.Error(), "signal: killed") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("error should mention kill/deadline, got %v", err)
	}
}

func TestTestContextZeroTimeoutNoDeadline(t *testing.T) {
	dir := initRepo(t)
	g := New(dir, "")
	out, err := g.TestContext("echo hi", 0)
	if err != nil {
		t.Fatalf("TestContext: %v", err)
	}
	if out != "hi" {
		t.Errorf("TestContext output = %q, want hi", out)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
