package command

import (
	"fmt"

	"xynapse/internal/config"
	"xynapse/internal/storage"
)

func GetSprint(cfg *config.Config) error {
	s := storage.NewStorage(cfg.Storage.Base)

	tickets, err := s.ReadSprintTickets(cfg.Defaults.Project)
	if err != nil {
		return err
	}

	if err := printTickets(cfg.Defaults.OutputFormat, tickets); err != nil {
		return fmt.Errorf("failed to print sprint tickets: %w", err)
	}
	return nil
}
