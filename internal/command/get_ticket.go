package command

import (
	"fmt"

	"xynapse/internal/config"
	"xynapse/internal/models"
	"xynapse/internal/storage"
)

func GetTicket(cfg *config.Config, ticketRef, format string) error {
	s := storage.NewStorage(cfg.Storage.Base)
	ticket, err := resolveTicket(cfg, s, ticketRef)
	if err != nil {
		return err
	}

	if format == "" {
		format = cfg.Defaults.OutputFormat
	}
	if err := printTickets(format, []*models.Ticket{ticket}); err != nil {
		return fmt.Errorf("failed to print ticket: %w", err)
	}
	return nil
}
