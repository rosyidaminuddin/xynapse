package command

import (
	"fmt"
	"os"

	"xynapse/internal/config"
	"xynapse/internal/storage"
)

// ShowPlan displays a ticket's saved implementation plan from
// <storage>/plans/<KEY>.md, rendered as markdown. The ticket does not need to
// be cached locally; only the plan file is read.
func ShowPlan(cfg *config.Config, ticketRef string) error {
	s := storage.NewStorage(cfg.Storage.Base)
	project, number, err := ParseTicketRef(ticketRef, cfg.Defaults.Project)
	if err != nil {
		return err
	}

	planPath := s.GetPlanPath(project, number)
	data, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("no plan found for %s-%s at %s (run `xynapse plan %s` first): %w", project, number, planPath, ticketRef, err)
	}
	if len(data) == 0 {
		return fmt.Errorf("plan %s is empty (run `xynapse plan %s` first)", planPath, ticketRef)
	}

	if msg, ok := staleWarning(s, project, number, ticketRef); ok {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}

	return RenderMD(os.Stdout, string(data))
}
