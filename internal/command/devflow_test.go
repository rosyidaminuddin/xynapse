package command

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"xynapse/internal/models"
	"xynapse/internal/storage"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

// initRemote creates a bare git repo to act as an origin.
func initRemote(t *testing.T) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, filepath.Dir(remote), "init", "-q", "--bare", remote)
	return remote
}

// initRepo creates a git repo in a temp dir with an initial commit on main
// and, when remote is non-empty, an origin remote pointing at a bare repo.
func initRepo(t *testing.T, remote string) string {
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
	if remote != "" {
		runGit(t, dir, "remote", "add", "origin", remote)
	}
	return dir
}

// writeTicket stores a ticket in the storage base so resolveTicket can read it.
func writeTicket(t *testing.T, base, key string) {
	t.Helper()
	s := storage.NewStorage(base)
	if err := s.WriteTicket(&models.Ticket{
		Key: key, Project: "PROJ", Summary: "Add stuff", Status: "Open",
	}); err != nil {
		t.Fatal(err)
	}
}

// stubGH writes a fake `gh` executable into a temp dir and prepends it to
// PATH. The stub records its arguments to the file named by GH_ARGS_FILE and
// prints a fixed PR URL.
func stubGH(t *testing.T, argsFile string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$GH_ARGS_FILE\"\necho \"https://github.com/example/xynapse/pull/1\"\n"
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_ARGS_FILE", argsFile)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
