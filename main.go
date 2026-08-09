package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"xynapse/internal/command"
	"xynapse/internal/config"
)

// configDir returns the per-user config directory for xynapse.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "xynapse"), nil
}

// resolveConfigPaths picks the config and env files to load. It prefers the
// project-local files (config/config.yaml and .env) when present, and falls
// back to the per-user directory (~/.config/xynapse/) so the globally
// installed binary works from anywhere.
func resolveConfigPaths() (configPath, envPath string) {
	if _, err := os.Stat("config/config.yaml"); err == nil {
		return "config/config.yaml", ".env"
	}
	dir, err := configDir()
	if err == nil {
		return filepath.Join(dir, "config.yaml"), filepath.Join(dir, ".env")
	}
	return "config/config.yaml", ".env"
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cfg := &config.Config{}
	var verbose bool
	var project string

	root := &cobra.Command{
		Use:   "xynapse",
		Short: "Manage Jira tickets locally",
		Long: `xynapse fetches Jira tickets and stores them as local YAML files
so get commands can read them without hitting the server.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			level := slog.LevelWarn
			if verbose {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

			configPath, envPath := resolveConfigPaths()
			loaded, err := config.Load(configPath, envPath)
			if err != nil {
				return err
			}
			*cfg = *loaded
			cfg.Verbose = verbose

			if project != "" {
				cfg.Defaults.Project = project
			}

			if dir := os.Getenv("XYNAPSE_STORAGE"); dir != "" {
				cfg.Storage.Base = dir
			}

			return cfg.Validate()
		},
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "log every step to stderr")
	root.PersistentFlags().StringVarP(&project, "project", "p", "", "override the default project key")

	root.AddCommand(
		newPullTicketCmd(cfg),
		newPullSprintCmd(cfg),
		newGetTicketCmd(cfg),
		newGetSprintCmd(cfg),
	)

	return root
}

func newPullTicketCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "pull-ticket <ticket-number|key|url>",
		Short: "fetch a ticket from Jira and write it as yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return command.PullTicket(cfg, args[0])
		},
	}
}

func newPullSprintCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull-sprint",
		Short: "fetch all tickets in the current sprint for the current user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			types, _ := cmd.Flags().GetStringSlice("type")
			return command.PullSprint(cfg, types)
		},
	}
	cmd.Flags().StringSliceP("type", "t", nil, "comma-separated issue types to filter by (e.g. Story,Bug,Epic)")
	return cmd
}

func newGetTicketCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "get-ticket <ticket-number|key|url>",
		Short: "read a ticket from local yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return command.GetTicket(cfg, args[0])
		},
	}
}

func newGetSprintCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-sprint",
		Short: "list all tickets from the active sprint",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			types, _ := cmd.Flags().GetStringSlice("type")
			return command.GetSprint(cfg, types)
		},
	}
	cmd.Flags().StringSliceP("type", "t", nil, "comma-separated issue types to filter by (e.g. Story,Bug,Epic)")
	return cmd
}
