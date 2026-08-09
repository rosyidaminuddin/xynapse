package command

import (
	"fmt"

	"xynapse/internal/config"
	"xynapse/internal/models"
	"xynapse/internal/storage"
)

func GetTicket(cfg *config.Config, ticketNum string) error {
	s := storage.NewStorage(cfg.Storage.Base)

	logStep(cfg.Verbose, "reading local ticket %s-%s from %s", cfg.Defaults.Project, ticketNum, cfg.Storage.Base)
	ticket, err := s.ReadTicket(cfg.Defaults.Project, ticketNum)
	if err != nil {
		return err
	}
	logStep(cfg.Verbose, "ticket %s loaded (status=%s, assignee=%s)", ticket.Key, ticket.Status, ticket.Assignee)

	if err := printTickets(cfg.Defaults.OutputFormat, []*models.Ticket{ticket}); err != nil {
		return fmt.Errorf("failed to print ticket: %w", err)
	}
	return nil
}
