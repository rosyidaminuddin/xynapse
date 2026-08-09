package command

import (
	"fmt"
	"strings"

	"xynapse/internal/config"
	"xynapse/internal/models"
	"xynapse/internal/storage"
)

func GetSprint(cfg *config.Config, types []string) error {
	s := storage.NewStorage(cfg.Storage.Base)

	logStep(cfg.Verbose, "reading sprint manifest for project %s from %s", cfg.Defaults.Project, cfg.Storage.Base)
	tickets, err := s.ReadSprintTickets(cfg.Defaults.Project)
	if err != nil {
		return err
	}
	logStep(cfg.Verbose, "loaded %d sprint ticket(s) from local storage", len(tickets))

	if len(types) > 0 {
		tickets = filterByType(tickets, types)
		logStep(cfg.Verbose, "filtered to %d ticket(s) of type(s) %v", len(tickets), types)
	}

	if err := printTickets(cfg.Defaults.OutputFormat, tickets); err != nil {
		return fmt.Errorf("failed to print sprint tickets: %w", err)
	}
	return nil
}

// filterByType keeps only tickets whose type matches one of the given types (case-insensitive).
func filterByType(tickets []*models.Ticket, types []string) []*models.Ticket {
	allowed := make(map[string]bool, len(types))
	for _, t := range types {
		allowed[strings.ToLower(strings.TrimSpace(t))] = true
	}

	filtered := make([]*models.Ticket, 0, len(tickets))
	for _, ticket := range tickets {
		if allowed[strings.ToLower(ticket.Type)] {
			filtered = append(filtered, ticket)
		}
	}
	return filtered
}
