package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubGH writes a fake `gh` executable into a temp dir and prepends it to
// PATH. The stub records its arguments to the file named by GH_ARGS_FILE and
// prints a fixed PR URL.
func stubGH(t *testing.T, argsFile string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$GH_ARGS_FILE\"\necho \"https://github.com/example/xynapse/pull/1\"\n"
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_ARGS_FILE", argsFile)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestCreatePR(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "gh_args.txt")
	stubGH(t, argsFile)

	url, err := CreatePR("gh", "", PROptions{
		Base:  "main",
		Head:  "feature-v5/PROJ-1",
		Title: "PROJ-1: Add stuff",
		Body:  "Closes PROJ-1",
	})
	if err != nil {
		t.Fatalf("CreatePR: %v", err)
	}
	if url != "https://github.com/example/xynapse/pull/1" {
		t.Errorf("url = %q", url)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.TrimSpace(string(data))
	for _, want := range []string{"pr", "create", "--base", "main", "--head", "feature-v5/PROJ-1", "--title", "PROJ-1: Add stuff", "--body", "Closes PROJ-1"} {
		if !strings.Contains(args, want) {
			t.Errorf("gh args missing %q: %q", want, args)
		}
	}
}

func TestCreatePRMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := CreatePR("gh-nonexistent", "", PROptions{Base: "main", Title: "t", Body: "b"}); err == nil {
		t.Error("expected error for missing gh binary")
	}
}

// stubGHOutput writes a fake `gh` executable that prints fixed output for any
// invocation, and prepends its dir to PATH.
func stubGHOutput(t *testing.T, output string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$GH_ARGS_FILE\"\nprintf '%s' " + quoteShell(output) + "\n"
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GH_ARGS_FILE", filepath.Join(dir, "gh_args.txt"))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// quoteShell single-quotes s for safe embedding in a sh script.
func quoteShell(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func TestPRStates(t *testing.T) {
	stubGHOutput(t, `[{"number":1,"headRefName":"feature-v5/PROJ-1","state":"MERGED","baseRefName":"main"},{"number":2,"headRefName":"fix-v5/PROJ-2","state":"OPEN","baseRefName":"develop"}]`)

	states, err := PRStates("gh", "")
	if err != nil {
		t.Fatalf("PRStates: %v", err)
	}
	if states["feature-v5/PROJ-1"].State != "merged" {
		t.Errorf("PRStates[feature-v5/PROJ-1].State = %q, want merged", states["feature-v5/PROJ-1"].State)
	}
	if states["feature-v5/PROJ-1"].Base != "main" {
		t.Errorf("PRStates[feature-v5/PROJ-1].Base = %q, want main", states["feature-v5/PROJ-1"].Base)
	}
	if states["fix-v5/PROJ-2"].State != "open" {
		t.Errorf("PRStates[fix-v5/PROJ-2].State = %q, want open", states["fix-v5/PROJ-2"].State)
	}
	if states["fix-v5/PROJ-2"].Base != "develop" {
		t.Errorf("PRStates[fix-v5/PROJ-2].Base = %q, want develop", states["fix-v5/PROJ-2"].Base)
	}
}

func TestPRStatesMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := PRStates("gh-nonexistent", ""); err == nil {
		t.Error("expected error for missing gh binary")
	}
}
