package command

import (
	"fmt"

	"xynapse/internal/config"
	"xynapse/internal/storage"
)

// PlanStatus shows a ticket's plan status, or updates it when set is non-empty.
// Valid statuses: not started, in progress, in review, done.
func PlanStatus(cfg *config.Config, ticketRef, set string) error {
	s := storage.NewStorage(cfg.Storage.Base)
	project, number, err := ParseTicketRef(ticketRef, cfg.Defaults.Project)
	if err != nil {
		return err
	}

	if set != "" {
		if err := s.SetPlanStatus(project, number, set); err != nil {
			return err
		}
		fmt.Printf("%s: %s\n", number, set)
		return nil
	}

	status, ok := s.PlanStatus(project, number)
	if !ok {
		return fmt.Errorf("no plan found for %s-%s (run `xynapse plan %s` first)", project, number, ticketRef)
	}
	fmt.Printf("%s: %s\n", number, status)
	return nil
}
