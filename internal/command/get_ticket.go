package command

import (
	"fmt"
	"time"

	"xynapse/internal/config"
	"xynapse/internal/models"
	"xynapse/internal/storage"
)

func GetTicket(cfg *config.Config, ticketRef, format string) error {
	project, number, err := ParseTicketRef(ticketRef, cfg.Defaults.Project)
	if err != nil {
		return err
	}

	s := storage.NewStorage(cfg.Storage.Base)

	logStep(cfg.Verbose, "reading local ticket %s-%s from %s", project, number, cfg.Storage.Base)
	ticket, err := s.ReadTicket(project, number)
	if err != nil {
		return err
	}
	logStep(cfg.Verbose, "ticket %s loaded (fetched_at=%s)", ticket.Key, ticket.FetchedAt.Format(time.RFC3339))

	if expired := cfg.Expiration.Duration(); expired > 0 && time.Since(ticket.FetchedAt) > expired {
		logStep(cfg.Verbose, "cached ticket is stale (older than %s), refreshing from Jira", expired)
		if err := PullTicket(cfg, ticketRef); err != nil {
			return err
		}
		ticket, err = s.ReadTicket(project, number)
		if err != nil {
			return err
		}
	}

	if format == "" {
		format = cfg.Defaults.OutputFormat
	}
	if err := printTickets(format, []*models.Ticket{ticket}); err != nil {
		return fmt.Errorf("failed to print ticket: %w", err)
	}
	return nil
}
