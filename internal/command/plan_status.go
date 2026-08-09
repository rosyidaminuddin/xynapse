package command

import (
	"fmt"
	"os"
	"time"

	"xynapse/internal/storage"
)

// Plan status values for the sprint PLAN column.
const (
	PlanNone  = "no"
	PlanFresh = "yes"
	PlanStale = "stale"
)

// planStatus reports whether a ticket has a saved plan and whether that plan
// is stale — the ticket was updated on Jira after the plan was written.
// Returns PlanNone, PlanFresh, or PlanStale.
func planStatus(s *storage.Storage, project, key string, updatedAt time.Time) string {
	modTime, ok := s.PlanModTime(project, key)
	if !ok {
		return PlanNone
	}
	if updatedAt.After(modTime) {
		return PlanStale
	}
	return PlanFresh
}

// staleWarning returns a human-readable warning when the cached ticket's plan
// is stale, or ok=false when there is no cached ticket to compare or the plan
// is current. The check is skipped entirely when the ticket is not cached, so
// plan-only workflows keep working offline.
func staleWarning(s *storage.Storage, project, number, ticketRef string) (msg string, ok bool) {
	ticket, err := s.ReadTicket(project, number)
	if err != nil {
		return "", false
	}
	if planStatus(s, project, ticket.Key, ticket.UpdatedAt) != PlanStale {
		return "", false
	}
	modTime, _ := s.PlanModTime(project, ticket.Key)
	return fmt.Sprintf("plan for %s is stale (ticket updated %s, plan written %s) — run `xynapse plan %s` to regenerate",
		ticket.Key, ticket.UpdatedAt.Format(time.RFC3339), modTime.Format(time.RFC3339), ticketRef), true
}

// confirmStale warns about a stale plan on stderr and, unless force is set,
// asks the user to confirm before proceeding.
func confirmStale(s *storage.Storage, project, number, ticketRef string, force bool) error {
	msg, ok := staleWarning(s, project, number, ticketRef)
	if !ok {
		return nil
	}
	fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
	if force {
		return nil
	}
	fmt.Print("Continue with the stale plan? [y/N] ")
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return fmt.Errorf("cancelled: %w", err)
	}
	if answer != "y" && answer != "Y" {
		return fmt.Errorf("cancelled")
	}
	return nil
}
