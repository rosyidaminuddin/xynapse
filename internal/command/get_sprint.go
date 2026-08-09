package command

import (
	"fmt"
	"strings"
	"time"

	"xynapse/internal/config"
	"xynapse/internal/models"
	"xynapse/internal/storage"
)

func GetSprint(cfg *config.Config, types []string) error {
	s := storage.NewStorage(cfg.Storage.Base)

	logStep(cfg.Verbose, "reading sprint manifest for project %s from %s", cfg.Defaults.Project, cfg.Storage.Base)
	manifest, err := s.ReadSprintManifest(cfg.Defaults.Project)
	if err != nil {
		return err
	}
	logStep(cfg.Verbose, "sprint manifest loaded (fetched_at=%s, %d tickets)", manifest.FetchedAt.Format(time.RFC3339), len(manifest.TicketKeys))

	if expired := cfg.Expiration.Duration(); expired > 0 && time.Since(manifest.FetchedAt) > expired {
		logStep(cfg.Verbose, "cached sprint is stale (older than %s), refreshing from Jira", expired)
		if err := PullSprint(cfg, types); err != nil {
			return err
		}
	}

	tickets, err := s.ReadSprintTickets(cfg.Defaults.Project)
	if err != nil {
		return err
	}

	if len(types) > 0 {
		tickets = filterByType(tickets, types)
		logStep(cfg.Verbose, "filtered to %d ticket(s) of type(s) %v", len(tickets), types)
	}

	if err := printTickets(cfg.Defaults.OutputFormat, tickets); err != nil {
		return fmt.Errorf("failed to print sprint tickets: %w", err)
	}
	return nil
}

// filterByType keeps only tickets whose type matches one of the given types
// (case-insensitive). An empty type list returns all tickets unchanged.
func filterByType(tickets []*models.Ticket, types []string) []*models.Ticket {
	if len(types) == 0 {
		return tickets
	}

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
