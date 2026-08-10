package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Jira     JiraConfig               `yaml:"jira"`
	Defaults Defaults                 `yaml:"defaults"`
	Storage  StorageConfig            `yaml:"storage"`
	Expiration Expiration             `yaml:"expiration"`
	Opencode OpencodeConfig           `yaml:"opencode"`
	// Git is the repository and branching configuration: dir locates the
	// project repository for all git/gh commands, branch_template and
	// branch_templates name feature branches. The top-level values act as
	// defaults; per-project overrides live under projects.<KEY>.git and are
	// merged on top by ResolveProject.
	Git      GitConfig                `yaml:"git"`
	Projects map[string]ProjectConfig `yaml:"projects"`
	Verbose  bool                     `yaml:"-"`
}

type JiraConfig struct {
	URL            string `yaml:"url"`
	Email          string `yaml:"email"`
	APIToken       string `yaml:"api_token"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

type Defaults struct {
	Project      string `yaml:"project"`
	BoardID      string `yaml:"board_id"`
	OutputFormat string `yaml:"output_format"`
}

// ProjectConfig holds per-project overrides, applied on top of the global
// config when that project is selected. Git fields that are left empty fall
// back to the top-level git config.
type ProjectConfig struct {
	BoardID string    `yaml:"board_id"`
	Git     GitConfig `yaml:"git"`
}

type StorageConfig struct {
	Base string `yaml:"base"`
}

// OpencodeConfig controls how xynapse invokes the opencode CLI to run skills.
type OpencodeConfig struct {
	Bin         string `yaml:"bin"`
	Model       string `yaml:"model"`
	AutoApprove bool   `yaml:"auto_approve"`
}

// Expiration controls how long locally cached tickets stay fresh before
// get commands auto-refresh them from the server.
type Expiration struct {
	Hours int `yaml:"hours"`
}

// GitConfig controls how xynapse drives git/gh for a project's prepare,
// finalize, and get-sprint status. Configured at the top level (defaults) and
// overridable per-project under projects.<KEY>.git.
type GitConfig struct {
	// Dir is the repository directory the git/gh checks run in and where
	// plan/implement/prepare/finalize operate. When empty, commands fall back
	// to the current working directory.
	Dir string `yaml:"dir"`
	// BranchTemplate expands to a feature branch name for a ticket. Supported
	// placeholders: {Key}/{TicketKey}, {Project}, {Number}, {Board}, {Summary}.
	// Used as the fallback when no per-type template matches.
	BranchTemplate string `yaml:"branch_template"`
	// BranchTemplates overrides BranchTemplate per issue type (keys are
	// matched case-insensitively, e.g. Story/Bug/Epic).
	BranchTemplates map[string]string `yaml:"branch_templates"`
}

// Duration returns the expiry window as a time.Duration (0 when disabled).
func (e Expiration) Duration() time.Duration {
	if e.Hours <= 0 {
		return 0
	}
	return time.Duration(e.Hours) * time.Hour
}

// Load reads the YAML config and overlays environment variables (including a local .env file).
func Load(path, envPath string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", path, err)
	}

	loadEnvFile(envPath)

	if v := os.Getenv("JIRA_URL"); v != "" {
		cfg.Jira.URL = v
	}
	if v := os.Getenv("JIRA_EMAIL"); v != "" {
		cfg.Jira.Email = v
	}
	if v := os.Getenv("JIRA_API_TOKEN"); v != "" {
		cfg.Jira.APIToken = v
	}
	if v := os.Getenv("JIRA_BOARD_ID"); v != "" {
		cfg.Defaults.BoardID = v
	}
	if v := os.Getenv("JIRA_PROJECT"); v != "" {
		cfg.Defaults.Project = v
	}

	if cfg.Jira.TimeoutSeconds == 0 {
		cfg.Jira.TimeoutSeconds = 15
	}
	if cfg.Defaults.OutputFormat == "" {
		cfg.Defaults.OutputFormat = "table"
	}
	if cfg.Storage.Base == "" {
		cfg.Storage.Base = "storage"
	}
	if cfg.Opencode.Bin == "" {
		cfg.Opencode.Bin = "opencode"
	}

	return &cfg, nil
}

// Validate checks that required configuration is present before any command runs.
func (c *Config) Validate() error {
	if c.Jira.URL == "" {
		return fmt.Errorf("config error: jira.url is required (set it in ~/.config/xynapse/config.yaml or JIRA_URL)")
	}
	if c.Jira.Email == "" {
		return fmt.Errorf("config error: jira.email is required (set it in ~/.config/xynapse/config.yaml or JIRA_EMAIL)")
	}
	if c.Jira.APIToken == "" {
		return fmt.Errorf("config error: jira.api_token is required (set it in ~/.config/xynapse/config.yaml or JIRA_API_TOKEN)")
	}
	if c.Defaults.Project == "" && len(c.Projects) == 0 {
		return fmt.Errorf("config error: defaults.project is required (or configure projects)")
	}
	return nil
}

// ExpandDir expands a leading "~" to the user's home directory and makes the
// path absolute against the process working directory. An empty input stays
// empty so callers can fall back to the current working directory.
func ExpandDir(dir string) string {
	if dir == "" {
		return ""
	}
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
		}
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return dir
}

// ResolveProject returns a copy of the config with the active project resolved
// and its per-project overrides applied. The project key is chosen in order of
// precedence: the -p/--project flag, defaults.project, or the single configured
// project. With multiple projects and no default it errors. A flag project that
// is not in the projects map is still accepted and falls back to the global
// settings. Per-project git fields override the top-level git section
// field-by-field; empty fields keep the top-level value.
func (c *Config) ResolveProject(flagProject string) (*Config, error) {
	key, err := c.effectiveProject(flagProject)
	if err != nil {
		return nil, err
	}

	resolved := *c
	resolved.Defaults.Project = key

	if p, ok := c.findProject(key); ok {
		if p.BoardID != "" {
			resolved.Defaults.BoardID = p.BoardID
		}
		if p.Git.Dir != "" {
			resolved.Git.Dir = p.Git.Dir
		}
		if p.Git.BranchTemplate != "" {
			resolved.Git.BranchTemplate = p.Git.BranchTemplate
		}
		if len(p.Git.BranchTemplates) > 0 {
			resolved.Git.BranchTemplates = p.Git.BranchTemplates
		}
	}
	if resolved.Git.BranchTemplate == "" {
		resolved.Git.BranchTemplate = "feature-v5/{Key}"
	}
	resolved.Git.Dir = ExpandDir(resolved.Git.Dir)
	return &resolved, nil
}

// effectiveProject picks the project key without applying any overrides.
func (c *Config) effectiveProject(flagProject string) (string, error) {
	if flagProject != "" {
		return flagProject, nil
	}
	if c.Defaults.Project != "" {
		return c.Defaults.Project, nil
	}
	if len(c.Projects) == 1 {
		for k := range c.Projects {
			return k, nil
		}
	}
	if len(c.Projects) > 1 {
		return "", fmt.Errorf("no default project configured; set defaults.project or pass -p/--project")
	}
	return "", fmt.Errorf("config error: defaults.project is required")
}

// findProject looks up a project by exact key first, then case-insensitively.
func (c *Config) findProject(key string) (ProjectConfig, bool) {
	if p, ok := c.Projects[key]; ok {
		return p, true
	}
	lower := strings.ToLower(strings.TrimSpace(key))
	for k, v := range c.Projects {
		if strings.ToLower(strings.TrimSpace(k)) == lower {
			return v, true
		}
	}
	return ProjectConfig{}, false
}

func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}
