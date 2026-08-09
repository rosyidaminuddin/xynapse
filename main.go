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

// configPaths returns the per-user config and env file paths for xynapse.
// Config is always loaded from ~/.config/xynapse/, never from project files.
func configPaths() (configPath, envPath string) {
	dir := os.Getenv("XYNAPSE_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", ""
		}
		dir = filepath.Join(home, ".config", "xynapse")
	}
	return filepath.Join(dir, "config.yaml"), filepath.Join(dir, ".env")
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
		Use:           "xynapse",
		Short:         "Manage Jira tickets locally",
		SilenceErrors: true, // main() prints the error once to stderr
		Long: `xynapse fetches Jira tickets and stores them as local YAML files
so get commands can read them without hitting the server.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			level := slog.LevelWarn
			if verbose {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

			configPath, envPath := configPaths()
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
		newClearCacheCmd(cfg),
		newPlanCmd(cfg),
		newImplementCmd(cfg),
		newShowPlanCmd(cfg),
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
	cmd := &cobra.Command{
		Use:   "get-ticket <ticket-number|key|url>",
		Short: "read a ticket from local yaml",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("output")
			return command.GetTicket(cfg, args[0], format)
		},
	}
	cmd.Flags().StringP("output", "o", "", "output format: table, json, or yaml (overrides config)")
	return cmd
}

func newPlanCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan <ticket-number|key|url>",
		Short: "analyze a ticket and generate an implementation plan via opencode",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := cmd.Flags().GetString("dir")
			if dir == "" {
				dir = cfg.Opencode.Dir
			}
			model, _ := cmd.Flags().GetString("model")
			if model == "" {
				model = cfg.Opencode.Model
			}
			return command.Plan(cfg, args[0], dir, model)
		},
	}
	cmd.Flags().String("dir", "", "target repo directory (default: current working directory)")
	cmd.Flags().String("model", "", "override the opencode model")
	return cmd
}

func newImplementCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "implement <ticket-number|key|url>",
		Short: "execute a ticket's implementation plan in the repo via opencode",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := cmd.Flags().GetString("dir")
			if dir == "" {
				dir = cfg.Opencode.Dir
			}
			model, _ := cmd.Flags().GetString("model")
			if model == "" {
				model = cfg.Opencode.Model
			}
			planPath, _ := cmd.Flags().GetString("plan")
			return command.Implement(cfg, args[0], planPath, dir, model)
		},
	}
	cmd.Flags().String("dir", "", "target repo directory (default: current working directory)")
	cmd.Flags().String("model", "", "override the opencode model")
	cmd.Flags().String("plan", "", "path to the plan file (default: <storage>/plans/<KEY>.md)")
	return cmd
}

func newShowPlanCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "show-plan <ticket-number|key|url>",
		Short: "display a ticket's saved implementation plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return command.ShowPlan(cfg, args[0])
		},
	}
}

func newClearCacheCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear-cache",
		Short: "delete locally cached tickets and plans",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			project, _ := cmd.Root().PersistentFlags().GetString("project")
			force, _ := cmd.Flags().GetBool("force")
			return command.ClearCache(cfg, project, force)
		},
	}
	cmd.Flags().BoolP("force", "f", false, "skip the confirmation prompt")
	return cmd
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
