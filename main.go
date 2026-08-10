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
			resolved, err := loaded.ResolveProject(project)
			if err != nil {
				return err
			}
			*cfg = *resolved
			cfg.Verbose = verbose

			if dir := os.Getenv("XYNAPSE_STORAGE"); dir != "" {
				cfg.Storage.Base = dir
			}

			return cfg.Validate()
		},
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "log every step to stderr")
	root.PersistentFlags().StringVarP(&project, "project", "p", "", "project key to use (defaults to defaults.project or the single configured project)")

	root.AddCommand(
		newPullTicketCmd(cfg),
		newPullSprintCmd(cfg),
		newGetTicketCmd(cfg),
		newGetSprintCmd(cfg),
		newClearCacheCmd(cfg),
		newPlanCmd(cfg),
		newImplementCmd(cfg),
		newShowPlanCmd(cfg),
		newPrepareCmd(cfg),
		newFinalizeCmd(cfg),
		newDriveCmd(cfg),
		newStatusCmd(cfg),
		newTransitionCmd(cfg),
		newAssigneeCmd(cfg),
		newConfigCmd(),
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
				dir = cfg.Git.Dir
			}
			dir = config.ExpandDir(dir)
			model, _ := cmd.Flags().GetString("model")
			if model == "" {
				model = cfg.Opencode.Model
			}
			branch, _ := cmd.Flags().GetString("branch")
			return command.Plan(cfg, args[0], dir, model, branch)
		},
	}
	cmd.Flags().String("dir", "", "target repo directory (default: config git.dir, then current working directory)")
	cmd.Flags().String("model", "", "override the opencode model")
	cmd.Flags().StringP("branch", "b", "", "check out this branch in the target repo before planning")
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
				dir = cfg.Git.Dir
			}
			dir = config.ExpandDir(dir)
			model, _ := cmd.Flags().GetString("model")
			if model == "" {
				model = cfg.Opencode.Model
			}
			planPath, _ := cmd.Flags().GetString("plan")
			force, _ := cmd.Flags().GetBool("force")
			return command.Implement(cfg, args[0], planPath, dir, model, force)
		},
	}
	cmd.Flags().String("dir", "", "target repo directory (default: config git.dir, then current working directory)")
	cmd.Flags().String("model", "", "override the opencode model")
	cmd.Flags().String("plan", "", "path to the plan file (default: <storage>/plans/<KEY>.md)")
	cmd.Flags().BoolP("force", "f", false, "skip the stale-plan confirmation prompt")
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
		Long: "List all tickets from the active sprint from local YAML. When the\n" +
			"configured git repo is available, the FINALIZED, PR, TARGET, and BRANCH\n" +
			"columns reflect local git state: FINALIZED is 'yes' when a local commit\n" +
			"subject contains <KEY>:, PR is the gh pull-request state\n" +
			"(merged/open/closed/none), TARGET is the PR's base branch, and BRANCH\n" +
			"is the branch matching the ticket (via the branch template, or scanned\n" +
			"by key). Outside a repo these columns show '?'.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			types, _ := cmd.Flags().GetStringSlice("type")
			unplanned, _ := cmd.Flags().GetBool("unplanned")
			return command.GetSprint(cfg, types, unplanned)
		},
	}
	cmd.Flags().StringSliceP("type", "t", nil, "comma-separated issue types to filter by (e.g. Story,Bug,Epic)")
	cmd.Flags().Bool("unplanned", false, "only list tickets without a saved plan")
	return cmd
}

func newPrepareCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare <ticket-number|key|url>",
		Short: "create a feature branch for a ticket from a base branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := cmd.Flags().GetString("dir")
			if dir == "" {
				dir = cfg.Git.Dir
			}
			dir = config.ExpandDir(dir)
			base, _ := cmd.Flags().GetString("base")
			template, _ := cmd.Flags().GetString("template")
			force, _ := cmd.Flags().GetBool("force")
			return command.Prepare(cfg, args[0], dir, base, template, force)
		},
	}
	cmd.Flags().String("dir", "", "target repo directory (default: config git.dir, then current working directory)")
	cmd.Flags().StringP("base", "b", "", "base branch to branch from (required)")
	cmd.Flags().String("template", "", "branch template (default: config git.branch_template)")
	cmd.Flags().BoolP("force", "f", false, "proceed even if the working tree is dirty")
	_ = cmd.MarkFlagRequired("base")
	return cmd
}

func newFinalizeCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "finalize <ticket-number|key|url>",
		Short: "commit changes, push the branch, and optionally open a pull request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := cmd.Flags().GetString("dir")
			if dir == "" {
				dir = cfg.Git.Dir
			}
			dir = config.ExpandDir(dir)
			base, _ := cmd.Flags().GetString("base")
			message, _ := cmd.Flags().GetString("message")
			pr, _ := cmd.Flags().GetBool("pr")
			return command.Finalize(cfg, args[0], dir, base, message, pr)
		},
	}
	cmd.Flags().String("dir", "", "target repo directory (default: config git.dir, then current working directory)")
	cmd.Flags().StringP("base", "b", "", "base branch (PR target; refuse to commit on it when set)")
	cmd.Flags().String("message", "", "commit message (default: \"<KEY>: <summary>\")")
	cmd.Flags().Bool("pr", false, "create a pull request with gh after pushing (requires --base)")
	return cmd
}

func newStatusCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <ticket-number|key|url>",
		Short: "show or update a ticket's plan status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			set, _ := cmd.Flags().GetString("set")
			return command.PlanStatus(cfg, args[0], set)
		},
	}
	cmd.Flags().String("set", "", "set the plan status (not started, in progress, in review, done)")
	return cmd
}

func newDriveCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drive <ticket-number|key|url>",
		Short: "drive a ticket through the whole workflow and update it on Jira",
		Long: "Drive a single ticket through the full pipeline: prepare a feature\n" +
			"branch, generate/refresh the plan, implement it, run the configured\n" +
			"tests and linter, finalize (commit, push, open a PR against\n" +
			"workflow.target_branch), and — once the PR is merged — update the\n" +
			"Jira ticket (assignee, transition to workflow.test_status, comment).\n\n" +
			"The workflow stops after the PR is created until it is merged; a\n" +
			"re-run resumes and checks the PR state before updating the ticket.\n" +
			"Steps are recorded per ticket, so interrupted runs resume where they\n" +
			"stopped. Without --yes every decision is prompted interactively.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := cmd.Flags().GetString("dir")
			if dir == "" {
				dir = cfg.Git.Dir
			}
			dir = config.ExpandDir(dir)
			model, _ := cmd.Flags().GetString("model")
			if model == "" {
				model = cfg.Opencode.Model
			}
			base, _ := cmd.Flags().GetString("base")
			status, _ := cmd.Flags().GetString("status")
			auto, _ := cmd.Flags().GetBool("yes")
			force, _ := cmd.Flags().GetBool("force")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			step, _ := cmd.Flags().GetString("step")
			from, _ := cmd.Flags().GetString("from")
			to, _ := cmd.Flags().GetString("to")
			return command.Drive(cfg, command.DriveOptions{
				TicketRef: args[0],
				Dir:       dir,
				Model:     model,
				Base:      base,
				Status:    status,
				Auto:      auto,
				Force:     force,
				DryRun:    dryRun,
				Step:      step,
				From:      from,
				To:        to,
			})
		},
	}
	cmd.Flags().String("dir", "", "target repo directory (default: config git.dir, then current working directory)")
	cmd.Flags().String("model", "", "override the opencode model")
	cmd.Flags().StringP("base", "b", "", "PR target branch (default: config workflow.target_branch, then base_branch)")
	cmd.Flags().String("status", "", "Jira status to move the ticket to after the PR merges (default: config workflow.test_status)")
	cmd.Flags().BoolP("yes", "y", false, "autopilot: answer confirmations with suggested defaults and skip prompts")
	cmd.Flags().BoolP("force", "f", false, "bypass failed tests/lint and the stale-plan prompt")
	cmd.Flags().Bool("dry-run", false, "print the steps drive would run without executing them")
	cmd.Flags().String("step", "", "run a single step (branch, plan, implement, test, lint, finalize, ticket)")
	cmd.Flags().String("from", "", "first step in the range to run")
	cmd.Flags().String("to", "", "last step in the range to run")
	return cmd
}

func newTransitionCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transition <ticket-number|key|url> [status]",
		Short: "move a Jira ticket to a new status (or list available transitions)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			transitionID, _ := cmd.Flags().GetString("id")
			statusName := ""
			if len(args) > 1 {
				statusName = args[1]
			}
			return command.Transition(cfg, args[0], statusName, transitionID)
		},
	}
	cmd.Flags().String("id", "", "transition id to force (disambiguates multiple transitions to the same status)")
	return cmd
}

func newAssigneeCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "assignee <ticket-number|key|url> [user]",
		Short: "show or update a ticket's assignee",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			user := ""
			if len(args) > 1 {
				user = args[1]
			}
			return command.Assignee(cfg, args[0], user)
		},
	}
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "get, set, and inspect the xynapse configuration",
		Args:  cobra.NoArgs,
		// Override the root PersistentPreRunE (which loads and validates the
		// config) — the config command manages the config file itself, so it
		// must work even before Jira credentials exist.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, envPath := configPaths()
			out, err := config.Dump(configPath, envPath)
			if err != nil {
				return err
			}
			fmt.Print(out)
			return nil
		},
	}
	cmd.AddCommand(newConfigGetCmd(), newConfigSetCmd(), newConfigPathCmd())
	return cmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "print the effective value of a config key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, envPath := configPaths()
			val, err := config.Get(configPath, envPath, args[0])
			if err != nil {
				return err
			}
			fmt.Println(val)
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "write a config value to the config file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := configPaths()
			changed, err := config.Set(configPath, args[0], args[1])
			if err != nil {
				return err
			}
			if changed {
				fmt.Printf("set %s to %s\n", args[0], args[1])
			} else {
				fmt.Printf("%s is already %s\n", args[0], args[1])
			}
			return nil
		},
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "print the config file path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := configPaths()
			fmt.Println(configPath)
			return nil
		},
	}
}
