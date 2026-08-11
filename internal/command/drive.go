package command

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"xynapse/internal/client"
	"xynapse/internal/config"
	"xynapse/internal/git"
	"xynapse/internal/models"
	"xynapse/internal/storage"
)

// Drive step names, in pipeline order.
const (
	stepBranch    = "branch"
	stepPlan      = "plan"
	stepImplement = "implement"
	stepTest      = "test"
	stepLint      = "lint"
	stepFinalize  = "finalize"
	stepTicket    = "ticket"
)

var driveSteps = []string{stepBranch, stepPlan, stepImplement, stepTest, stepLint, stepFinalize, stepTicket}

// DriveOptions configures a single `drive` invocation.
type DriveOptions struct {
	TicketRef string
	Dir       string
	Model     string
	// Base overrides the PR target branch (workflow.target_branch, else
	// workflow.base_branch).
	Base string
	// Status overrides the workflow.test_status the ticket moves to.
	Status string
	// Auto enables autopilot: confirmations use their suggested defaults and
	// interactive prompts are skipped.
	Auto bool
	// Force bypasses failed test/lint gates and the stale-plan prompt.
	Force bool
	// DryRun prints the steps drive would run without executing them.
	DryRun bool
	// Step runs a single step; From/To run an inclusive range.
	Step string
	From string
	To   string
}

// Drive drives a single ticket through the whole workflow: prepare the branch,
// plan, implement, run tests/lint, finalize (commit/push/PR), and — once the
// PR is merged — update the Jira ticket (assignee, transition to test status,
// comment). Steps are gated by drive state so re-running resumes where the
// previous run stopped.
func Drive(cfg *config.Config, opts DriveOptions) error {
	project, number, err := ParseTicketRef(opts.TicketRef, cfg.Defaults.Project)
	if err != nil {
		return err
	}

	s := storage.NewStorage(cfg.Storage.Base)
	if _, err := s.ReadTicket(project, number); err != nil {
		logStep(cfg.Verbose, "ticket %s-%s not cached, pulling from Jira", project, number)
		if err := PullTicket(cfg, opts.TicketRef); err != nil {
			return err
		}
	}
	ticket, err := resolveTicket(cfg, s, opts.TicketRef)
	if err != nil {
		return err
	}

	dir := opts.Dir
	if dir == "" {
		dir = cfg.Git.Dir
	}
	dir = config.ExpandDir(dir)

	g := git.New(dir, "")
	inside, err := g.IsInsideWorkTree()
	if err != nil || !inside {
		return fmt.Errorf("%s is not inside a git working tree; check git.dir in the config", dir)
	}

	auto := opts.Auto || (cfg.Workflow.Autopilot != nil && *cfg.Workflow.Autopilot)

	steps, err := selectedSteps(opts)
	if err != nil {
		return err
	}

	if opts.DryRun {
		fmt.Printf("drive %s would run steps: %s\n", ticket.Key, strings.Join(steps, ", "))
		return nil
	}

	prevAuto := autoConfirm
	autoConfirm = auto
	defer func() { autoConfirm = prevAuto }()

	ctx := &driveCtx{cfg: cfg, s: s, g: g, opts: opts, auto: auto}

	for _, step := range steps {
		done, err := s.DriveStepDone(ticket.Project, ticket.Key, step)
		if err != nil {
			return err
		}
		if done && step != stepTicket {
			logStep(cfg.Verbose, "step %s already done, skipping", step)
			continue
		}

		fmt.Printf("== %s: %s\n", strings.ToUpper(step), ticket.Key)
		run, err := driveStep(ctx, step, ticket)
		if err != nil {
			return err
		}
		if !run {
			continue
		}
		if err := s.SetDriveStep(ticket.Project, ticket.Key, step); err != nil {
			return err
		}
	}
	return nil
}

type driveCtx struct {
	cfg  *config.Config
	s    *storage.Storage
	g    *git.Git
	opts DriveOptions
	auto bool
}

// driveStep runs a single pipeline step and reports whether it completed (and
// should be recorded). A false return means the step was intentionally skipped
// (e.g. the plan already exists and is fresh) or is waiting on a gate (the PR
// is not merged yet).
func driveStep(ctx *driveCtx, step string, ticket *models.Ticket) (bool, error) {
	switch step {
	case stepBranch:
		return driveBranchStep(ctx, ticket)
	case stepPlan:
		return drivePlanStep(ctx, ticket)
	case stepImplement:
		return driveImplementStep(ctx, ticket)
	case stepTest:
		return driveTestStep(ctx, ticket, stepTest)
	case stepLint:
		if ctx.cfg.Workflow.LintCommand == "" {
			logStep(ctx.cfg.Verbose, "workflow.lint_command is empty, skipping lint")
			return false, nil
		}
		return driveTestStep(ctx, ticket, stepLint)
	case stepFinalize:
		return driveFinalizeStep(ctx, ticket)
	case stepTicket:
		return driveTicketStep(ctx, ticket)
	}
	return false, fmt.Errorf("unknown step %q", step)
}

// selectedSteps returns the ordered step list the invocation should run,
// honoring --step/--from/--to. When nothing is filtered, all steps run.
func selectedSteps(opts DriveOptions) ([]string, error) {
	if opts.Step != "" {
		if !containsString(driveSteps, opts.Step) {
			return nil, fmt.Errorf("unknown step %q (valid: %s)", opts.Step, strings.Join(driveSteps, ", "))
		}
		return []string{opts.Step}, nil
	}

	start, end := 0, len(driveSteps)
	if opts.From != "" {
		i := indexOf(driveSteps, opts.From)
		if i < 0 {
			return nil, fmt.Errorf("unknown step %q (valid: %s)", opts.From, strings.Join(driveSteps, ", "))
		}
		start = i
	}
	if opts.To != "" {
		i := indexOf(driveSteps, opts.To)
		if i < 0 {
			return nil, fmt.Errorf("unknown step %q (valid: %s)", opts.To, strings.Join(driveSteps, ", "))
		}
		end = i + 1
	}
	if start > end {
		return nil, fmt.Errorf("--from %q comes after --to %q", opts.From, opts.To)
	}
	return driveSteps[start:end], nil
}

func indexOf(xs []string, s string) int {
	for i, x := range xs {
		if x == s {
			return i
		}
	}
	return -1
}

// derivedBranch expands the configured branch template for a ticket, mirroring
// Prepare's logic so PR lookup and branch creation agree.
func derivedBranch(cfg *config.Config, ticket *models.Ticket) (string, error) {
	tmpl := git.ResolveTemplate(cfg.Git.BranchTemplate, cfg.Git.BranchTemplates, ticket.Type)
	if tmpl == "" {
		tmpl = "feature-v5/{Key}"
	}
	return git.ExpandTemplate(tmpl, git.TemplateVars{
		Key:     ticket.Key,
		Project: ticket.Project,
		Number:  ticketNumber(ticket.Key),
		Board:   cfg.Defaults.BoardID,
		Summary: ticket.Summary,
	})
}

// drivePRTarget resolves the PR base branch: --base, else
// workflow.target_branch, else workflow.base_branch. Empty means no PR target
// is configured.
func drivePRTarget(cfg *config.Config, flagBase string) string {
	if flagBase != "" {
		return flagBase
	}
	if cfg.Workflow.TargetBranch != "" {
		return cfg.Workflow.TargetBranch
	}
	return cfg.Workflow.BaseBranch
}

// testStatus resolves the ticket's target test status: --status, else
// workflow.test_status. driveTicketStep requires it to be set.
func testStatus(cfg *config.Config, flagStatus string) string {
	if flagStatus != "" {
		return flagStatus
	}
	return cfg.Workflow.TestStatus
}

func driveBranchStep(ctx *driveCtx, ticket *models.Ticket) (bool, error) {
	branch, err := derivedBranch(ctx.cfg, ticket)
	if err != nil {
		return false, err
	}
	if ctx.g.BranchExists(branch) {
		logStep(ctx.cfg.Verbose, "branch %s already exists", branch)
		if err := ctx.g.Checkout(branch); err != nil {
			return false, fmt.Errorf("failed to check out %s: %w", branch, err)
		}
		fmt.Printf("On branch %s\n", branch)
		return true, nil
	}

	base := ctx.opts.Base
	if base == "" {
		base = ctx.cfg.Workflow.BaseBranch
	}
	if base == "" {
		return false, fmt.Errorf("workflow.base_branch is not configured; set it or pass --base")
	}
	if err := Prepare(ctx.cfg, ctx.opts.TicketRef, ctx.opts.Dir, base, "", ctx.opts.Force); err != nil {
		return false, err
	}
	return true, nil
}

func drivePlanStep(ctx *driveCtx, ticket *models.Ticket) (bool, error) {
	status := planStatus(ctx.s, ticket.Project, ticket.Key, ticket.UpdatedAt)
	if status == PlanFresh {
		logStep(ctx.cfg.Verbose, "plan exists and is fresh, skipping")
		return false, nil
	}

	if status == PlanStale && !ctx.auto && !ctx.opts.Force {
		ok, err := askYesNo(fmt.Sprintf("Plan for %s is stale. Regenerate it?", ticket.Key), confirmReader)
		if err != nil {
			return false, err
		}
		if !ok {
			fmt.Printf("Skipping plan for %s; the stale plan will be used.\n", ticket.Key)
			return false, nil
		}
	}

	if err := Plan(ctx.cfg, ctx.opts.TicketRef, ctx.opts.Dir, ctx.opts.Model, ""); err != nil {
		return false, err
	}
	return true, nil
}

func driveImplementStep(ctx *driveCtx, ticket *models.Ticket) (bool, error) {
	if !ctx.s.HasPlan(ticket.Project, ticket.Key) {
		return false, fmt.Errorf("no plan saved for %s; run `xynapse plan %s` (or drive) first", ticket.Key, ctx.opts.TicketRef)
	}
	if err := Implement(ctx.cfg, ctx.opts.TicketRef, "", ctx.opts.Dir, ctx.opts.Model, ctx.auto || ctx.opts.Force); err != nil {
		return false, err
	}
	return true, nil
}

// driveTestStep runs workflow.test_command (or lint_command) and reports
// whether it completed. A failing run records the failure and — without
// --force — stops the pipeline so finalize cannot proceed.
func driveTestStep(ctx *driveCtx, ticket *models.Ticket, step string) (bool, error) {
	var command, label string
	if step == stepLint {
		command = ctx.cfg.Workflow.LintCommand
		label = "lint"
	} else {
		command = ctx.cfg.Workflow.TestCommand
		label = "tests"
	}
	if strings.TrimSpace(command) == "" {
		logStep(ctx.cfg.Verbose, "workflow.%s_command is empty, skipping", label)
		return false, nil
	}

	fmt.Printf("Running %s: %s\n", label, command)
	out, err := ctx.g.Test(command)
	if strings.TrimSpace(out) != "" {
		fmt.Println(strings.TrimSpace(out))
	}
	if err != nil {
		_ = ctx.s.SetDriveFailure(ticket.Project, ticket.Key, step, err.Error())
		if ctx.opts.Force {
			fmt.Printf("warning: %s failed, but --force bypasses the gate\n", label)
			return false, nil
		}
		return false, fmt.Errorf("%s failed: %w", label, err)
	}
	return true, nil
}

func driveFinalizeStep(ctx *driveCtx, ticket *models.Ticket) (bool, error) {
	if msg, failed := ctx.s.DriveStepFailed(ticket.Project, ticket.Key, stepTest); failed && !ctx.opts.Force {
		return false, fmt.Errorf("cannot finalize: tests failed (%s); pass --force to override", msg)
	}
	if msg, failed := ctx.s.DriveStepFailed(ticket.Project, ticket.Key, stepLint); failed && !ctx.opts.Force {
		return false, fmt.Errorf("cannot finalize: lint failed (%s); pass --force to override", msg)
	}

	target := drivePRTarget(ctx.cfg, ctx.opts.Base)
	if target == "" {
		return false, fmt.Errorf("no PR target branch configured; set workflow.target_branch (or base_branch), or pass --base")
	}

	if err := Finalize(ctx.cfg, ctx.opts.TicketRef, ctx.opts.Dir, target, "", true); err != nil {
		return false, err
	}
	fmt.Printf("Waiting for PR to be merged before updating ticket %s.\n", ticket.Key)
	fmt.Printf("Re-run `xynapse drive %s` once the PR is merged.\n", ctx.opts.TicketRef)
	return true, nil
}

// driveTicketStep runs only after the PR is merged. Until then it prints the
// waiting reason and returns false (not completed), so the drive state is not
// marked done and a re-run re-checks the gate.
func driveTicketStep(ctx *driveCtx, ticket *models.Ticket) (bool, error) {
	status := testStatus(ctx.cfg, ctx.opts.Status)
	if status == "" {
		return false, fmt.Errorf("workflow.test_status is not configured; set it or pass --status")
	}

	branch, err := derivedBranch(ctx.cfg, ticket)
	if err != nil {
		return false, err
	}
	pr, err := git.PRView("gh", ctx.opts.Dir, branch)
	if err != nil {
		return false, err
	}

	switch pr.State {
	case "merged":
		// proceed below
	case "none":
		fmt.Printf("No pull request found for %s on branch %s.\n", ticket.Key, branch)
		return false, nil
	case "open":
		switch pr.ReviewDecision {
		case "APPROVED":
			fmt.Printf("PR #%d for %s is approved but not yet merged — merge it, then re-run `xynapse drive %s`.\n", pr.Number, ticket.Key, ctx.opts.TicketRef)
		case "CHANGES_REQUESTED":
			fmt.Printf("Changes were requested on PR #%d for %s — fix the branch, then re-run `xynapse drive %s`.\n", pr.Number, ticket.Key, ctx.opts.TicketRef)
		default:
			fmt.Printf("PR #%d for %s is awaiting review — re-run `xynapse drive %s` once it is merged.\n", pr.Number, ticket.Key, ctx.opts.TicketRef)
		}
		return false, nil
	default:
		fmt.Printf("PR #%d for %s is %s; re-run `xynapse drive %s` once it is merged.\n", pr.Number, ticket.Key, pr.State, ctx.opts.TicketRef)
		return false, nil
	}

	if strings.EqualFold(ticket.Status, status) {
		fmt.Printf("%s is already in %s; nothing to do.\n", ticket.Key, status)
		return true, nil
	}

	if err := driveAssignee(ctx, ticket); err != nil {
		return false, err
	}
	comment, err := driveComment(ctx, ticket, pr)
	if err != nil {
		return false, err
	}

	if err := Transition(ctx.cfg, ctx.opts.TicketRef, status, ""); err != nil {
		return false, err
	}
	if comment != "" {
		if err := addJiraComment(ctx.cfg, ticket, comment); err != nil {
			return false, err
		}
	}
	return true, nil
}

// driveAssignee asks who to assign the ticket to before the transition, unless
// autopilot keeps the current assignee. Empty input keeps the current assignee.
func driveAssignee(ctx *driveCtx, ticket *models.Ticket) error {
	if ctx.auto {
		logStep(ctx.cfg.Verbose, "autopilot: keeping current assignee for %s", ticket.Key)
		return nil
	}
	user := askLine(fmt.Sprintf("Who should %s be assigned to? (name, email, or unassigned; Enter keeps current): ", ticket.Key), confirmReader)
	if strings.TrimSpace(user) == "" {
		return nil
	}
	return Assignee(ctx.cfg, ctx.opts.TicketRef, user)
}

// driveComment decides the Jira comment to post: a user-provided comment when
// given, otherwise the workflow.comment_template. Autopilot always uses the
// template. An empty result posts no comment.
func driveComment(ctx *driveCtx, ticket *models.Ticket, pr *git.PRDetails) (string, error) {
	if ctx.auto {
		return renderCommentTemplate(ctx.cfg, ticket, pr), nil
	}
	answer := askLine(fmt.Sprintf("Comment to post on %s? (Enter to use the template, or type a custom comment): ", ticket.Key), confirmReader)
	if strings.TrimSpace(answer) != "" {
		return answer, nil
	}
	return renderCommentTemplate(ctx.cfg, ticket, pr), nil
}

// renderCommentTemplate expands the configured comment template, replacing
// {key}, {summary}, {url}, and {branch}. The PR head branch is derived from the
// ticket so the template can reference it even when PRDetails lacks the head.
func renderCommentTemplate(cfg *config.Config, ticket *models.Ticket, pr *git.PRDetails) string {
	tmpl := cfg.Workflow.CommentTemplate
	if strings.TrimSpace(tmpl) == "" {
		tmpl = "PR: {url}\n\nCloses {key}"
	}
	branch, err := derivedBranch(cfg, ticket)
	if err != nil {
		branch = ""
	}
	r := strings.NewReplacer(
		"{key}", ticket.Key,
		"{summary}", ticket.Summary,
		"{url}", pr.URL,
		"{branch}", branch,
	)
	return r.Replace(tmpl)
}

// askYesNo prompts with [y/N] and reports whether the answer was yes.
func askYesNo(prompt string, r io.Reader) (bool, error) {
	fmt.Printf("%s [y/N] ", prompt)
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("cancelled: %w", err)
		}
		return false, fmt.Errorf("cancelled: no answer provided")
	}
	ans := strings.ToLower(strings.TrimSpace(scanner.Text()))
	return ans == "y" || ans == "yes", nil
}

// askLine prompts and reads one line of input.
func askLine(prompt string, r io.Reader) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}

// addJiraComment posts the comment to the ticket via the Jira API.
func addJiraComment(cfg *config.Config, ticket *models.Ticket, comment string) error {
	c := client.NewJiraClient(cfg.Jira.URL, cfg.Jira.Email, cfg.Jira.APIToken, cfg.Jira.TimeoutSeconds)
	if err := c.AddComment(ticket.Project, ticketNumber(ticket.Key), comment); err != nil {
		return err
	}
	fmt.Printf("Commented on %s\n", ticket.Key)
	return nil
}
