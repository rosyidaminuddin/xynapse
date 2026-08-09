package command

import (
	"fmt"
	"os"

	"xynapse/internal/config"
	"xynapse/internal/opencode"
	"xynapse/internal/storage"
)

// Implement runs the implement-plan skill via opencode, executing a saved
// plan in the target repository. The plan is loaded from --plan or, by
// default, from <storage>/plans/<KEY>.md. When the plan is stale — the
// ticket changed after it was written — the user is asked to confirm unless
// force is set.
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

	if err := s.SetPlanStatus(project, number, storage.PlanStatusInProgress); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not mark plan in progress: %v\n", err)
	}

	prompt := fmt.Sprintf("Use the implement-plan skill. Execute this plan in the current repository, following its steps and acceptance criteria.\n\nPlan:\n%s", string(planBytes))

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
