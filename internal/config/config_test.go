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
  dir: "/work/repo"

git:
  branch_template: "feature-v5/{Key}"
  branch_templates:
    Bug: "fix-v5/{Key}"
    Epic: "epic/{Key}"

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
	if cfg.Opencode.Dir != "/work/repo" {
		t.Errorf("Opencode.Dir = %q", cfg.Opencode.Dir)
	}
	if cfg.Git.BranchTemplate != "feature-v5/{Key}" {
		t.Errorf("Git.BranchTemplate = %q, want feature-v5/{Key}", cfg.Git.BranchTemplate)
	}
	if cfg.Git.BranchTemplates["Bug"] != "fix-v5/{Key}" || cfg.Git.BranchTemplates["Epic"] != "epic/{Key}" {
		t.Errorf("Git.BranchTemplates = %v", cfg.Git.BranchTemplates)
	}
	if cfg.Projects["MERADIO"].BoardID != "561" {
		t.Errorf("Projects[MERADIO].BoardID = %q", cfg.Projects["MERADIO"].BoardID)
	}
	if cfg.Projects["MERADIO"].Git.BranchTemplates["Epic"] != "epic-v5/{Key}" {
		t.Errorf("Projects[MERADIO] git = %v", cfg.Projects["MERADIO"].Git)
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
	if cfg.Git.BranchTemplate != "feature-v5/{Key}" {
		t.Errorf("Git.BranchTemplate default = %q, want feature-v5/{Key}", cfg.Git.BranchTemplate)
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
				BranchTemplate:  "feature-v5/{Key}",
				BranchTemplates: map[string]string{"Bug": "fix-v5/{Key}"},
			},
			Projects: map[string]ProjectConfig{
				"ALPHA": {BoardID: "99", Git: GitConfig{BranchTemplate: "release/{Key}"}},
				"BETA":  {BoardID: "42", Git: GitConfig{BranchTemplates: map[string]string{"Epic": "epic/{Key}"}}},
			},
		}
	}

	t.Run("flag wins", func(t *testing.T) {
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
		if cfg.Git.BranchTemplate != "feature-v5/{Key}" {
			t.Errorf("BranchTemplate should fall back to global, got %q", cfg.Git.BranchTemplate)
		}
		if cfg.Git.BranchTemplates["Epic"] != "epic/{Key}" {
			t.Errorf("BranchTemplates = %v", cfg.Git.BranchTemplates)
		}
		if cfg.Git.BranchTemplates["Bug"] != "" {
			t.Errorf("per-project branch_templates should replace the global map, got %v", cfg.Git.BranchTemplates)
		}
	})

	t.Run("default project", func(t *testing.T) {
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

	t.Run("free-form flag falls back to global", func(t *testing.T) {
		cfg, err := base().ResolveProject("ZETA")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Defaults.Project != "ZETA" || cfg.Defaults.BoardID != "" {
			t.Errorf("project=%q board=%q", cfg.Defaults.Project, cfg.Defaults.BoardID)
		}
		if cfg.Git.BranchTemplate != "feature-v5/{Key}" {
			t.Errorf("BranchTemplate = %q", cfg.Git.BranchTemplate)
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
