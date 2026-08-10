package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const validConfig = `jira:
  url: "https://example.atlassian.net"
  email: "user@example.com"
  api_token: "token123"
  timeout_seconds: 10

defaults:
  project: "PROJ"
  board_id: "123"
  output_format: "yaml"

storage:
  base: "data"

expiration:
  hours: 48

opencode:
  bin: "opencode"
  model: "anthropic/claude-sonnet-4"
  auto_approve: true

git:
  dir: "/work/repo"
  branch_template: "feature-v5/{Key}"
  branch_templates:
    Bug: "fix-v5/{Key}"

workflow:
  test_command: "go test ./..."
  lint_command: "go vet ./..."
  test_status: "In Review"
  base_branch: "main"
  comment_template: "PR: {url}"
  autopilot: true

projects:
  MERADIO:
    board_id: "561"
    git:
      branch_template: "feature-v5/{Key}"
      branch_templates:
        Bug: "fix-v5/{Key}"
        Epic: "epic-v5/{Key}"
  ALPHA:
    board_id: "99"
    git:
      branch_template: "release/{Key}"
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

func TestLoadValid(t *testing.T) {
	path := writeConfig(t, validConfig)
	cfg, err := Load(path, filepath.Join(filepath.Dir(path), ".env"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Jira.URL != "https://example.atlassian.net" {
		t.Errorf("URL = %q", cfg.Jira.URL)
	}
	if cfg.Jira.TimeoutSeconds != 10 {
		t.Errorf("TimeoutSeconds = %d, want 10", cfg.Jira.TimeoutSeconds)
	}
	if cfg.Defaults.Project != "PROJ" {
		t.Errorf("Project = %q", cfg.Defaults.Project)
	}
	if cfg.Defaults.OutputFormat != "yaml" {
		t.Errorf("OutputFormat = %q", cfg.Defaults.OutputFormat)
	}
	if cfg.Storage.Base != "data" {
		t.Errorf("Storage.Base = %q", cfg.Storage.Base)
	}
	if cfg.Expiration.Hours != 48 {
		t.Errorf("Expiration.Hours = %d, want 48", cfg.Expiration.Hours)
	}
	if got := cfg.Expiration.Duration(); got != 48*time.Hour {
		t.Errorf("Expiration.Duration() = %v, want %v", got, 48*time.Hour)
	}
	if cfg.Opencode.Bin != "opencode" {
		t.Errorf("Opencode.Bin = %q", cfg.Opencode.Bin)
	}
	if cfg.Opencode.Model != "anthropic/claude-sonnet-4" {
		t.Errorf("Opencode.Model = %q", cfg.Opencode.Model)
	}
	if !cfg.Opencode.AutoApprove {
		t.Errorf("Opencode.AutoApprove = false, want true")
	}
	if cfg.Git.Dir != "/work/repo" {
		t.Errorf("Git.Dir = %q, want /work/repo (loaded from top-level git)", cfg.Git.Dir)
	}
	if cfg.Git.BranchTemplate != "feature-v5/{Key}" {
		t.Errorf("Git.BranchTemplate = %q, want feature-v5/{Key}", cfg.Git.BranchTemplate)
	}
	if cfg.Git.BranchTemplates["Bug"] != "fix-v5/{Key}" {
		t.Errorf("Git.BranchTemplates = %v", cfg.Git.BranchTemplates)
	}
	if cfg.Projects["MERADIO"].BoardID != "561" {
		t.Errorf("Projects[MERADIO].BoardID = %q", cfg.Projects["MERADIO"].BoardID)
	}
	if cfg.Projects["MERADIO"].Git.BranchTemplates["Epic"] != "epic-v5/{Key}" {
		t.Errorf("Projects[MERADIO] git = %v", cfg.Projects["MERADIO"].Git)
	}

	// Workflow block parses.
	if cfg.Workflow.TestCommand != "go test ./..." {
		t.Errorf("Workflow.TestCommand = %q", cfg.Workflow.TestCommand)
	}
	if cfg.Workflow.LintCommand != "go vet ./..." {
		t.Errorf("Workflow.LintCommand = %q", cfg.Workflow.LintCommand)
	}
	if cfg.Workflow.TestStatus != "In Review" {
		t.Errorf("Workflow.TestStatus = %q", cfg.Workflow.TestStatus)
	}
	if cfg.Workflow.BaseBranch != "main" {
		t.Errorf("Workflow.BaseBranch = %q", cfg.Workflow.BaseBranch)
	}
	if cfg.Workflow.CommentTemplate != "PR: {url}" {
		t.Errorf("Workflow.CommentTemplate = %q", cfg.Workflow.CommentTemplate)
	}
	if cfg.Workflow.Autopilot == nil || !*cfg.Workflow.Autopilot {
		t.Errorf("Workflow.Autopilot = %v, want true", cfg.Workflow.Autopilot)
	}
}

func TestResolveProjectWorkflowMerge(t *testing.T) {
	doc := `workflow:
  test_command: "go test ./..."
  test_status: "QA"
  base_branch: "main"
  target_branch: "develop"
  comment_template: "top template"
  autopilot: false

projects:
  ALPHA:
    board_id: "99"
    workflow:
      test_command: "make test"
      comment_template: "alpha"
`
	path := writeConfig(t, doc)
	cfg, err := Load(path, filepath.Join(filepath.Dir(path), ".env"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// ALPHA overrides test_command and comment_template; the rest inherit.
	resolved, err := cfg.ResolveProject("ALPHA")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if resolved.Workflow.TestCommand != "make test" {
		t.Errorf("TestCommand = %q, want make test", resolved.Workflow.TestCommand)
	}
	if resolved.Workflow.TestStatus != "QA" {
		t.Errorf("TestStatus = %q, want QA", resolved.Workflow.TestStatus)
	}
	if resolved.Workflow.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want main", resolved.Workflow.BaseBranch)
	}
	if resolved.Workflow.TargetBranch != "develop" {
		t.Errorf("TargetBranch = %q, want develop", resolved.Workflow.TargetBranch)
	}
	if resolved.Workflow.CommentTemplate != "alpha" {
		t.Errorf("CommentTemplate = %q, want alpha", resolved.Workflow.CommentTemplate)
	}
	if resolved.Workflow.Autopilot == nil || *resolved.Workflow.Autopilot {
		t.Errorf("Autopilot = %v, want false", resolved.Workflow.Autopilot)
	}
}

func TestResolveProjectTargetBranchFallback(t *testing.T) {
	doc := `defaults:
  project: "PROJ"
workflow:
  base_branch: "main"
`
	path := writeConfig(t, doc)
	cfg, err := Load(path, filepath.Join(filepath.Dir(path), ".env"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	resolved, err := cfg.ResolveProject("")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if resolved.Workflow.TargetBranch != "main" {
		t.Errorf("TargetBranch = %q, want fallback to base_branch main", resolved.Workflow.TargetBranch)
	}
}

func TestResolveProjectAutopilotNilInherited(t *testing.T) {
	// A project that doesn't set autopilot keeps the top-level value.
	doc := `workflow:
  autopilot: true
projects:
  ALPHA:
    board_id: "99"
`
	path := writeConfig(t, doc)
	cfg, err := Load(path, filepath.Join(filepath.Dir(path), ".env"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	resolved, err := cfg.ResolveProject("ALPHA")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if resolved.Workflow.Autopilot == nil || !*resolved.Workflow.Autopilot {
		t.Errorf("Autopilot = %v, want inherited true", resolved.Workflow.Autopilot)
	}
}

func TestLoadDefaults(t *testing.T) {
	path := writeConfig(t, `jira:
  url: "https://example.atlassian.net"
  email: "user@example.com"
  api_token: "token123"
`)
	cfg, err := Load(path, filepath.Join(filepath.Dir(path), ".env"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Jira.TimeoutSeconds != 15 {
		t.Errorf("TimeoutSeconds default = %d, want 15", cfg.Jira.TimeoutSeconds)
	}
	if cfg.Defaults.OutputFormat != "table" {
		t.Errorf("OutputFormat default = %q, want table", cfg.Defaults.OutputFormat)
	}
	if cfg.Storage.Base != "storage" {
		t.Errorf("Storage.Base default = %q, want storage", cfg.Storage.Base)
	}
	if cfg.Expiration.Duration() != 0 {
		t.Errorf("Expiration.Duration() should be 0 when unset, got %v", cfg.Expiration.Duration())
	}
	if cfg.Opencode.Bin != "opencode" {
		t.Errorf("Opencode.Bin default = %q, want opencode", cfg.Opencode.Bin)
	}
	if cfg.Opencode.AutoApprove {
		t.Errorf("Opencode.AutoApprove default = true, want false")
	}
	if cfg.Git.BranchTemplate != "" {
		t.Errorf("Git.BranchTemplate default = %q, want empty", cfg.Git.BranchTemplate)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("JIRA_URL", "https://env.atlassian.net")
	t.Setenv("JIRA_API_TOKEN", "env-token")

	path := writeConfig(t, validConfig)
	cfg, err := Load(path, filepath.Join(filepath.Dir(path), ".env"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Jira.URL != "https://env.atlassian.net" {
		t.Errorf("URL = %q, want env override", cfg.Jira.URL)
	}
	if cfg.Jira.APIToken != "env-token" {
		t.Errorf("APIToken = %q, want env override", cfg.Jira.APIToken)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"), ".env")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		cfg  *Config
		want bool
	}{
		{"valid", &Config{
			Jira:     JiraConfig{URL: "https://x", Email: "e@e.com", APIToken: "t"},
			Defaults: Defaults{Project: "PROJ"},
		}, true},
		{"missing url", &Config{
			Jira:     JiraConfig{Email: "e@e.com", APIToken: "t"},
			Defaults: Defaults{Project: "PROJ"},
		}, false},
		{"missing email", &Config{
			Jira:     JiraConfig{URL: "https://x", APIToken: "t"},
			Defaults: Defaults{Project: "PROJ"},
		}, false},
		{"missing token", &Config{
			Jira:     JiraConfig{URL: "https://x", Email: "e@e.com"},
			Defaults: Defaults{Project: "PROJ"},
		}, false},
		{"missing project and no projects", &Config{
			Jira:     JiraConfig{URL: "https://x", Email: "e@e.com", APIToken: "t"},
			Defaults: Defaults{},
		}, false},
		{"projects configured without default", &Config{
			Jira:     JiraConfig{URL: "https://x", Email: "e@e.com", APIToken: "t"},
			Defaults: Defaults{},
			Projects: map[string]ProjectConfig{"PROJ": {}},
		}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err == nil) != tc.want {
				t.Errorf("Validate() = %v, want ok=%v", err, tc.want)
			}
		})
	}
}

func TestResolveProject(t *testing.T) {
	base := func() *Config {
		return &Config{
			Defaults: Defaults{Project: "ALPHA", OutputFormat: "table"},
			Git: GitConfig{
				Dir:             "/global",
				BranchTemplate:  "global/{Key}",
				BranchTemplates: map[string]string{"Bug": "fix-global/{Key}"},
			},
			Projects: map[string]ProjectConfig{
				"ALPHA": {BoardID: "99", Git: GitConfig{Dir: "/work/alpha", BranchTemplate: "release/{Key}"}},
				"BETA":  {BoardID: "42", Git: GitConfig{BranchTemplates: map[string]string{"Epic": "epic/{Key}"}}},
			},
		}
	}

	t.Run("flag wins with merge", func(t *testing.T) {
		cfg, err := base().ResolveProject("BETA")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Defaults.Project != "BETA" {
			t.Errorf("project = %q, want BETA", cfg.Defaults.Project)
		}
		if cfg.Defaults.BoardID != "42" {
			t.Errorf("BoardID = %q, want 42", cfg.Defaults.BoardID)
		}
		// BETA leaves Dir and BranchTemplate empty: they inherit top-level git.
		if cfg.Git.Dir != "/global" {
			t.Errorf("Dir = %q, want /global (inherited from top-level)", cfg.Git.Dir)
		}
		if cfg.Git.BranchTemplate != "global/{Key}" {
			t.Errorf("BranchTemplate = %q, want global/{Key} (inherited from top-level)", cfg.Git.BranchTemplate)
		}
		// BETA sets its own per-type map: it replaces the top-level map.
		if cfg.Git.BranchTemplates["Epic"] != "epic/{Key}" {
			t.Errorf("BranchTemplates = %v", cfg.Git.BranchTemplates)
		}
		if cfg.Git.BranchTemplates["Bug"] != "" {
			t.Errorf("per-project branch_templates should replace the top-level map, got %v", cfg.Git.BranchTemplates)
		}
	})

	t.Run("default project overrides", func(t *testing.T) {
		cfg, err := base().ResolveProject("")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Defaults.Project != "ALPHA" {
			t.Errorf("project = %q, want ALPHA", cfg.Defaults.Project)
		}
		if cfg.Defaults.BoardID != "99" {
			t.Errorf("BoardID = %q, want 99", cfg.Defaults.BoardID)
		}
		if cfg.Git.BranchTemplate != "release/{Key}" {
			t.Errorf("BranchTemplate = %q, want release/{Key}", cfg.Git.BranchTemplate)
		}
		if cfg.Git.Dir != "/work/alpha" {
			t.Errorf("Dir = %q, want /work/alpha", cfg.Git.Dir)
		}
		// ALPHA leaves the per-type map empty: it inherits the top-level map.
		if cfg.Git.BranchTemplates["Bug"] != "fix-global/{Key}" {
			t.Errorf("BranchTemplates = %v, want top-level Bug inherited", cfg.Git.BranchTemplates)
		}
	})

	t.Run("case-insensitive flag", func(t *testing.T) {
		cfg, err := base().ResolveProject("beta")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Defaults.Project != "beta" || cfg.Defaults.BoardID != "42" {
			t.Errorf("project=%q board=%q, want beta/42", cfg.Defaults.Project, cfg.Defaults.BoardID)
		}
	})

	t.Run("free-form flag uses top-level git", func(t *testing.T) {
		cfg, err := base().ResolveProject("ZETA")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Defaults.Project != "ZETA" || cfg.Defaults.BoardID != "" {
			t.Errorf("project=%q board=%q", cfg.Defaults.Project, cfg.Defaults.BoardID)
		}
		if cfg.Git.BranchTemplate != "global/{Key}" {
			t.Errorf("BranchTemplate = %q, want global/{Key}", cfg.Git.BranchTemplate)
		}
		if cfg.Git.Dir != "/global" {
			t.Errorf("Dir = %q, want /global", cfg.Git.Dir)
		}
	})

	t.Run("no projects and no top-level git defaults", func(t *testing.T) {
		cfg := &Config{Defaults: Defaults{Project: "SOLO"}}
		resolved, err := cfg.ResolveProject("")
		if err != nil {
			t.Fatal(err)
		}
		if resolved.Git.BranchTemplate != "feature-v5/{Key}" {
			t.Errorf("BranchTemplate = %q, want feature-v5/{Key}", resolved.Git.BranchTemplate)
		}
		if resolved.Git.Dir != "" {
			t.Errorf("Dir = %q, want empty", resolved.Git.Dir)
		}
	})

	t.Run("source config untouched", func(t *testing.T) {
		src := base()
		if _, err := src.ResolveProject("BETA"); err != nil {
			t.Fatal(err)
		}
		if src.Defaults.Project != "ALPHA" {
			t.Errorf("source Defaults.Project mutated to %q", src.Defaults.Project)
		}
	})
}

func TestResolveProjectSingleAutoDefault(t *testing.T) {
	cfg := &Config{
		Projects: map[string]ProjectConfig{"SOLO": {BoardID: "7"}},
	}
	resolved, err := cfg.ResolveProject("")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if resolved.Defaults.Project != "SOLO" {
		t.Errorf("project = %q, want SOLO", resolved.Defaults.Project)
	}
	if resolved.Defaults.BoardID != "7" {
		t.Errorf("BoardID = %q, want 7", resolved.Defaults.BoardID)
	}
}

func TestResolveProjectAmbiguous(t *testing.T) {
	cfg := &Config{
		Projects: map[string]ProjectConfig{
			"A": {BoardID: "1"},
			"B": {BoardID: "2"},
		},
	}
	if _, err := cfg.ResolveProject(""); err == nil {
		t.Fatal("expected error for multiple projects without a default")
	}
}

func TestResolveProjectCustomFieldsMerge(t *testing.T) {
	doc := `jira:
  url: "https://example.atlassian.net"
  email: "user@example.com"
  api_token: "token123"
  custom_fields:
    acceptance_criteria: "customfield_10001"
    story_points: "customfield_10002"

projects:
  MERADIO:
    board_id: "561"
    custom_fields:
      acceptance_criteria: "customfield_20001"
`
	path := writeConfig(t, doc)
	cfg, err := Load(path, filepath.Join(filepath.Dir(path), ".env"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.AcceptanceCriteriaField() != "customfield_10001" {
		t.Errorf("AcceptanceCriteriaField = %q, want customfield_10001", cfg.AcceptanceCriteriaField())
	}

	// MERADIO overrides acceptance_criteria; story_points inherits top-level.
	resolved, err := cfg.ResolveProject("MERADIO")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if resolved.Jira.CustomFields["acceptance_criteria"] != "customfield_20001" {
		t.Errorf("acceptance_criteria = %q, want per-project override customfield_20001", resolved.Jira.CustomFields["acceptance_criteria"])
	}
	if resolved.Jira.CustomFields["story_points"] != "customfield_10002" {
		t.Errorf("story_points = %q, want inherited customfield_10002", resolved.Jira.CustomFields["story_points"])
	}
	if resolved.AcceptanceCriteriaField() != "customfield_20001" {
		t.Errorf("AcceptanceCriteriaField = %q, want customfield_20001", resolved.AcceptanceCriteriaField())
	}

	// A project with no custom_fields keeps the top-level map intact.
	solo, err := cfg.ResolveProject("NOPE")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}
	if solo.AcceptanceCriteriaField() != "customfield_10001" {
		t.Errorf("AcceptanceCriteriaField = %q, want customfield_10001 for unconfigured project", solo.AcceptanceCriteriaField())
	}
}

func TestExpandDir(t *testing.T) {
	t.Run("empty stays empty", func(t *testing.T) {
		if got := ExpandDir(""); got != "" {
			t.Errorf("ExpandDir(\"\") = %q, want empty", got)
		}
	})

	t.Run("tilde expands to home", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skipf("no home dir: %v", err)
		}
		if got := ExpandDir("~"); got != home {
			t.Errorf("ExpandDir(\"~\") = %q, want %q", got, home)
		}
		if got := ExpandDir("~/work/repo"); got != filepath.Join(home, "work", "repo") {
			t.Errorf("ExpandDir(\"~/work/repo\") = %q, want %q", got, filepath.Join(home, "work", "repo"))
		}
	})

	t.Run("relative becomes absolute", func(t *testing.T) {
		want, err := filepath.Abs("work/repo")
		if err != nil {
			t.Fatal(err)
		}
		if got := ExpandDir("work/repo"); got != want {
			t.Errorf("ExpandDir(\"work/repo\") = %q, want %q", got, want)
		}
	})

	t.Run("absolute stays absolute", func(t *testing.T) {
		if got := ExpandDir("/abs/path"); got != "/abs/path" {
			t.Errorf("ExpandDir(\"/abs/path\") = %q, want /abs/path", got)
		}
	})
}

func TestResolveProjectExpandsGitDir(t *testing.T) {
	cfg := &Config{
		Defaults: Defaults{Project: "ALPHA"},
		Git:      GitConfig{Dir: "~/work/repo"},
	}
	resolved, err := cfg.ResolveProject("")
	if err != nil {
		t.Fatal(err)
	}
	home, herr := os.UserHomeDir()
	if herr != nil {
		t.Skipf("no home dir: %v", herr)
	}
	want := filepath.Join(home, "work", "repo")
	if resolved.Git.Dir != want {
		t.Errorf("Git.Dir = %q, want %q (tilde expanded)", resolved.Git.Dir, want)
	}
}
