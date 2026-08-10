package command

import (
	"fmt"
	"io"
	"os"

	"xynapse/internal/config"
	"xynapse/internal/opencode"
	"xynapse/internal/storage"
)

// confirmReader is the input source for plan confirmation prompts. Overridable
// in tests.
var confirmReader io.Reader = os.Stdin

// Implement runs the implement-plan skill via opencode, executing a saved
// plan in the target repository. The plan is loaded from --plan or, by
// default, from <storage>/plans/<KEY>.md. Before opencode runs, any unanswered
// confirmation from the plan's `## Confirmations` section is prompted for and
// recorded back into the plan. When the plan is stale — the ticket changed
// after it was written — the user is asked to confirm unless force is set.
func Implement(cfg *config.Config, ticketRef, planPath, dir, model string, force bool) error {
	s := storage.NewStorage(cfg.Storage.Base)
	project, number, err := ParseTicketRef(ticketRef, cfg.Defaults.Project)
	if err != nil {
		return err
	}

	if err := confirmStale(s, project, number, ticketRef, force); err != nil {
		return err
	}

	if planPath == "" {
		planPath = s.GetPlanPath(project, number)
	}
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("failed to read plan %s: %w (run `xynapse plan %s` first)", planPath, err, ticketRef)
	}
	if len(planBytes) == 0 {
		return fmt.Errorf("plan %s is empty (run `xynapse plan %s` first)", planPath, ticketRef)
	}

	planText, err := resolveConfirmations(s, project, number, planPath, string(planBytes))
	if err != nil {
		return err
	}

	if err := s.SetPlanStatus(project, number, storage.PlanStatusInProgress); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not mark plan in progress: %v\n", err)
	}

	prompt := fmt.Sprintf("Use the implement-plan skill. Execute this plan in the current repository, following its steps and acceptance criteria.\n\nPlan:\n%s", planText)

	logStep(cfg.Verbose, "running opencode implement-plan skill (dir=%s)", dir)
	out, err := opencode.Run(opencode.Options{
		Bin:         cfg.Opencode.Bin,
		Dir:         dir,
		Model:       model,
		AutoApprove: cfg.Opencode.AutoApprove,
		Prompt:      prompt,
		Stream:      os.Stderr,
	})
	if err != nil {
		return err
	}
	report := opencode.ExtractText(out)
	if err := RenderMD(os.Stdout, report); err != nil {
		return fmt.Errorf("failed to render report: %w", err)
	}
	return nil
}

// resolveConfirmations checks the plan for unanswered confirmation questions
// and, when there are any, prompts the user for answers and records them in a
// `## Decisions` section. It returns the plan text to hand to the implementer
// and aborts with an error if the user cancels before answering.
func resolveConfirmations(s *storage.Storage, project, number, planPath, planText string) (string, error) {
	confirmations := parseConfirmations(planText)
	if len(confirmations) == 0 {
		return planText, nil
	}

	need := unanswered(confirmations, parseDecisions(planText))
	if len(need) == 0 {
		return planText, nil
	}

	fmt.Fprintf(os.Stderr, "plan requires %d confirmation(s) before implementing. Answer each question; press Enter to accept the suggested default.\n\n", len(need))
	for _, c := range need {
		fmt.Fprintf(os.Stderr, "  %d. %s\n", c.Number, c.Question)
	}
	fmt.Fprintln(os.Stderr)

	decisions, err := promptForAnswers(need, confirmReader, os.Stdout)
	if err != nil {
		return "", err
	}
	answered := writeDecisions(planText, decisions)

	if err := persistPlanBody(s, project, number, planPath, answered); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save confirmation answers: %v\n", err)
	}
	return answered, nil
}

// persistPlanBody saves the updated plan text back to disk, preserving the
// frontmatter for plans in the default storage location.
func persistPlanBody(s *storage.Storage, project, number, planPath, body string) error {
	if planPath == s.GetPlanPath(project, number) {
		return s.RewritePlanBody(project, number, body)
	}
	return os.WriteFile(planPath, []byte(body), 0o644)
}
