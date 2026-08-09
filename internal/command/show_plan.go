package command

import (
	"fmt"
	"os"
	"strings"

	"xynapse/internal/config"
	"xynapse/internal/storage"
)

// ShowPlan displays a ticket's saved implementation plan from
// <storage>/plans/<KEY>.md, rendered as markdown. The plan's lifecycle status
// (from the frontmatter) is shown as a header. The ticket does not need to be
// cached locally; only the plan file is read.
func ShowPlan(cfg *config.Config, ticketRef string) error {
	s := storage.NewStorage(cfg.Storage.Base)
	project, number, err := ParseTicketRef(ticketRef, cfg.Defaults.Project)
	if err != nil {
		return err
	}

	body, status, ok := s.ReadPlan(project, number)
	if !ok {
		return fmt.Errorf("no plan found for %s-%s at %s (run `xynapse plan %s` first)",
			project, number, s.GetPlanPath(project, number), ticketRef)
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("plan for %s-%s is empty (run `xynapse plan %s` first)", project, number, ticketRef)
	}

	if msg, ok := staleWarning(s, project, number, ticketRef); ok {
		fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	}

	md := fmt.Sprintf("**Status:** %s\n\n%s", status, body)
	return RenderMD(os.Stdout, md)
}
