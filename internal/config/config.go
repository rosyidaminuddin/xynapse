package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Jira     JiraConfig     `yaml:"jira"`
	Defaults Defaults       `yaml:"defaults"`
	Storage  StorageConfig  `yaml:"storage"`
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

type StorageConfig struct {
	Base string `yaml:"base"`
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

	if cfg.Jira.TimeoutSeconds == 0 {
		cfg.Jira.TimeoutSeconds = 15
	}
	if cfg.Defaults.OutputFormat == "" {
		cfg.Defaults.OutputFormat = "table"
	}
	if cfg.Storage.Base == "" {
		cfg.Storage.Base = "storage"
	}

	return &cfg, nil
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
