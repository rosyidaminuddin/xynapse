package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xynapse/internal/config"
	"xynapse/internal/models"
	"xynapse/internal/storage"
)

// stubGitGh writes fake `git` and `gh` executables onto PATH. The git stub
// responds to the three commands get-sprint runs; everything else fails so
// unexpected invocations surface in tests. gh prints prJSON for any call.
func stubGitGh(t *testing.T, inside, branches, subjects, prJSON string) {
	t.Helper()
	dir := t.TempDir()

	gitScript := `#!/bin/sh
case "$1 $2" in
  "rev-parse --is-inside-work-tree") echo "$GIT_STUB_INSIDE" ;;
  "for-each-ref --format=%(refname:short)") printf '%s' "$GIT_STUB_BRANCHES" ;;
  "log --all") printf '%s' "$GIT_STUB_SUBJECTS" ;;
  *) echo "unexpected git: $*" >&2; exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(gitScript), 0o755); err != nil {
		t.Fatal(err)
	}

	ghScript := "#!/bin/sh\nprintf '%s' \"$GH_STUB_PR\"\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(ghScript), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GIT_STUB_INSIDE", inside)
	t.Setenv("GIT_STUB_BRANCHES", branches)
	t.Setenv("GIT_STUB_SUBJECTS", subjects)
	t.Setenv("GH_STUB_PR", prJSON)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func seedSprint(t *testing.T, base string, tickets []*models.Ticket) {
	t.Helper()
	s := storage.NewStorage(base)
	if err := s.WriteSprintManifest("PROJ", 0, "Sprint 1", tickets); err != nil {
		t.Fatal(err)
	}
}

func sprintConfig(base, repoDir string) *config.Config {
	cfg := testConfig(base)
	cfg.Git = config.GitConfig{Dir: repoDir, BranchTemplate: "feature-v5/{Key}"}
	return cfg
}

func sprintJSON(t *testing.T, out string) []map[string]any {
	t.Helper()
	var views []map[string]any
	if err := json.Unmarshal([]byte(out), &views); err != nil {
		t.Fatalf("invalid sprint JSON: %v\n%s", err, out)
	}
	return views
}

func assertSprintFields(t *testing.T, v map[string]any, want map[string]string) {
	t.Helper()
	for k, w := range want {
		if v[k] != w {
			t.Errorf("%s = %v, want %v", k, v[k], w)
		}
	}
}

func TestGetSprintGitStatus(t *testing.T) {
	stubGitGh(t, "true",
		"main\nfeature-v5/PROJ-1\ncustom/PROJ-3\n",
		"PROJ-1: Add stuff\nPROJ-3: WIP\ninit\n",
		`[{"number":1,"headRefName":"feature-v5/PROJ-1","state":"MERGED","baseRefName":"main"},{"number":3,"headRefName":"custom/PROJ-3","state":"OPEN","baseRefName":"develop"}]`)

	base := t.TempDir()
	seedSprint(t, base, []*models.Ticket{
		{Key: "PROJ-1", Project: "PROJ", Status: "Open", Type: "Story", Summary: "S1"},
		{Key: "PROJ-2", Project: "PROJ", Status: "Open", Type: "Story", Summary: "S2"},
		{Key: "PROJ-3", Project: "PROJ", Status: "Open", Type: "Story", Summary: "S3"},
	})
	cfg := sprintConfig(base, t.TempDir())
	cfg.Defaults.OutputFormat = "json"

	out := captureStdout(t, func() {
		if err := GetSprint(cfg, nil, nil, false); err != nil {
			t.Fatalf("GetSprint: %v", err)
		}
	})

	views := sprintJSON(t, out)
	got := map[string]map[string]any{}
	for _, v := range views {
		got[v["Key"].(string)] = v
	}

	assertSprintFields(t, got["PROJ-1"], map[string]string{"finalized": "yes", "pr": "merged", "target": "main", "branch": "feature-v5/PROJ-1"})
	assertSprintFields(t, got["PROJ-2"], map[string]string{"finalized": "no", "pr": "none", "target": "-", "branch": "-"})
	// PROJ-3's branch was created with a --template override: scan finds it,
	// and PR lookup uses the actual branch.
	assertSprintFields(t, got["PROJ-3"], map[string]string{"finalized": "yes", "pr": "open", "target": "develop", "branch": "custom/PROJ-3"})
}

func TestGetSprintTableColumns(t *testing.T) {
	stubGitGh(t, "true", "feature-v5/PROJ-1\n", "PROJ-1: stuff\n", `[{"number":1,"headRefName":"feature-v5/PROJ-1","state":"MERGED","baseRefName":"main"}]`)
	base := t.TempDir()
	seedSprint(t, base, []*models.Ticket{
		{Key: "PROJ-1", Project: "PROJ", Status: "Open", Type: "Story", Summary: "S1"},
	})
	cfg := sprintConfig(base, t.TempDir())

	out := captureStdout(t, func() {
		if err := GetSprint(cfg, nil, nil, false); err != nil {
			t.Fatalf("GetSprint: %v", err)
		}
	})
	for _, want := range []string{"KEY", "STATUS", "FINALIZED", "PR", "TARGET", "BRANCH", "PLAN", "TYPE", "ASSIGNEE", "SUMMARY", "merged", "main", "feature-v5/PROJ-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}

func TestGetSprintFromOutsideRepoDir(t *testing.T) {
	stubGitGh(t, "true",
		"main\nfeature-v5/PROJ-1\n",
		"PROJ-1: Add stuff\ninit\n",
		`[{"number":1,"headRefName":"feature-v5/PROJ-1","state":"MERGED","baseRefName":"main"}]`)

	repoDir := t.TempDir()
	base := t.TempDir()
	seedSprint(t, base, []*models.Ticket{
		{Key: "PROJ-1", Project: "PROJ", Status: "Open", Type: "Story", Summary: "S1"},
	})

	// Simulate the user running get-sprint from a directory that is NOT the
	// project repo. The configured git.dir must still drive the git/gh checks.
	outside := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := sprintConfig(base, repoDir)
	cfg.Defaults.OutputFormat = "json"

	out := captureStdout(t, func() {
		if err := GetSprint(cfg, nil, nil, false); err != nil {
			t.Fatalf("GetSprint: %v", err)
		}
	})
	views := sprintJSON(t, out)
	assertSprintFields(t, views[0], map[string]string{"finalized": "yes", "pr": "merged", "target": "main", "branch": "feature-v5/PROJ-1"})
}

func TestGetSprintNotARepo(t *testing.T) {
	stubGitGh(t, "false", "", "", "")
	base := t.TempDir()
	seedSprint(t, base, []*models.Ticket{
		{Key: "PROJ-1", Project: "PROJ", Status: "Open", Type: "Story", Summary: "S1"},
	})
	cfg := sprintConfig(base, t.TempDir())
	cfg.Defaults.OutputFormat = "json"

	out := captureStdout(t, func() {
		if err := GetSprint(cfg, nil, nil, false); err != nil {
			t.Fatalf("GetSprint: %v", err)
		}
	})
	views := sprintJSON(t, out)
	assertSprintFields(t, views[0], map[string]string{"finalized": "?", "pr": "?", "target": "?", "branch": "?"})
}
