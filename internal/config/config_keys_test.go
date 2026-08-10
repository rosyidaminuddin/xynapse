package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const keysFixture = `# Jira
jira:
  url: "https://x.atlassian.net"
  email: "e@e.com"
  api_token: "secret-token"

defaults:
  project: "ALPHA"
  board_id: "99"

expiration:
  hours: 24

git:
  branch_templates:
    Bug: "fix-v5/{Key}"

projects:
  MERADIO:
    board_id: "561"
    git:
      branch_templates:
        Bug: "fix-v5/{Key}"
        Epic: "epic-v5/{Key}"
`

func TestGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigFile(t, path, keysFixture)

	cases := []struct{ key, want string }{
		{"jira.url", "https://x.atlassian.net"},
		{"defaults.project", "ALPHA"},
		{"defaults.board_id", "99"},
		{"expiration.hours", "24"},
		{"git.branch_templates.Bug", "fix-v5/{Key}"},
		{"projects.MERADIO.board_id", "561"},
		{"projects.MERADIO.git.branch_templates.Epic", "epic-v5/{Key}"},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			got, err := Get(path, "", tc.key)
			if err != nil {
				t.Fatalf("Get(%q): %v", tc.key, err)
			}
			if got != tc.want {
				t.Errorf("Get(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestGetAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigFile(t, path, keysFixture)

	got, err := Get(path, "", "git.branch_template")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "feature-v5/{Key}" {
		t.Errorf("git.branch_template = %q, want default feature-v5/{Key}", got)
	}
}

func TestGetEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigFile(t, path, "defaults:\n  project: ALPHA\n")
	t.Setenv("JIRA_PROJECT", "ZETA")

	got, err := Get(path, "", "defaults.project")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "ZETA" {
		t.Errorf("defaults.project = %q, want ZETA (env override)", got)
	}
}

func TestGetMissingKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigFile(t, path, keysFixture)

	if _, err := Get(path, "", "nope.not_here"); err == nil {
		t.Fatal("expected error for missing key")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v", err)
	}
}

func TestGetMidLevelKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigFile(t, path, keysFixture)

	got, err := Get(path, "", "git")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(got, "branch_templates") || !strings.Contains(got, "fix-v5/{Key}") {
		t.Errorf("git section = %q", got)
	}
}

func TestSetOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigFile(t, path, "defaults:\n  project: ALPHA\n")

	changed, err := Set(path, "defaults.project", "BETA")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !changed {
		t.Error("expected change")
	}

	cfg, err := Load(path, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Defaults.Project != "BETA" {
		t.Errorf("project = %q, want BETA", cfg.Defaults.Project)
	}

	changed, err = Set(path, "defaults.project", "BETA")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if changed {
		t.Error("expected no change for identical value")
	}
}

func TestSetCreatesNested(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigFile(t, path, "# existing\njira:\n  url: x\n")

	if _, err := Set(path, "projects.MERADIO.board_id", "99"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := Set(path, "opencode.model", "anthropic/claude-sonnet-4"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	cfg, err := Load(path, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Projects["MERADIO"].BoardID != "99" {
		t.Errorf("projects.MERADIO.board_id = %q, want 99", cfg.Projects["MERADIO"].BoardID)
	}
	if cfg.Opencode.Model != "anthropic/claude-sonnet-4" {
		t.Errorf("opencode.model = %q", cfg.Opencode.Model)
	}
}

func TestSetCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dir", "config.yaml")

	changed, err := Set(path, "defaults.project", "NEW")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if !changed {
		t.Error("expected change")
	}
	cfg, err := Load(path, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Defaults.Project != "NEW" {
		t.Errorf("project = %q, want NEW", cfg.Defaults.Project)
	}
}

func TestSetTypedValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigFile(t, path, "expiration:\n  hours: 24\n")

	for _, s := range [][2]string{
		{"expiration.hours", "48"},
		{"opencode.auto_approve", "true"},
		{"defaults.board_id", "561"},
		{"jira.url", "https://example.com"},
	} {
		if _, err := Set(path, s[0], s[1]); err != nil {
			t.Fatalf("Set(%q): %v", s[0], err)
		}
	}

	cfg, err := Load(path, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Expiration.Hours != 48 {
		t.Errorf("hours = %d, want 48", cfg.Expiration.Hours)
	}
	if !cfg.Opencode.AutoApprove {
		t.Error("auto_approve = false, want true")
	}
	if cfg.Defaults.BoardID != "561" {
		t.Errorf("board_id = %q, want 561", cfg.Defaults.BoardID)
	}
	if cfg.Jira.URL != "https://example.com" {
		t.Errorf("url = %q", cfg.Jira.URL)
	}
}

func TestSetPreservesComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "# top comment\ndefaults:\n  project: ALPHA # inline\n"
	writeConfigFile(t, path, content)

	if _, err := Set(path, "defaults.project", "BETA"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "top comment") {
		t.Errorf("top comment lost:\n%s", out)
	}
	if !strings.Contains(out, "inline") {
		t.Errorf("inline comment lost:\n%s", out)
	}
}

func TestDumpRedactsToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigFile(t, path, "jira:\n  api_token: \"secret-token\"\n")

	out, err := Dump(path, "")
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if !strings.Contains(out, "REDACTED") {
		t.Errorf("token not redacted:\n%s", out)
	}
	if strings.Contains(out, "secret-token") {
		t.Errorf("token leaked:\n%s", out)
	}
}

func TestDumpNoTokenLeavesUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeConfigFile(t, path, "jira:\n  url: x\n")

	out, err := Dump(path, "")
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if strings.Contains(out, "REDACTED") {
		t.Errorf("should not redact an empty token:\n%s", out)
	}
}
