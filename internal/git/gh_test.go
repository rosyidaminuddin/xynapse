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
