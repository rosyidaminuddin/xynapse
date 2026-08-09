package command

import (
	"fmt"

	"xynapse/internal/config"
	"xynapse/internal/storage"
)

func ClearCache(cfg *config.Config, project string, force bool) error {
	s := storage.NewStorage(cfg.Storage.Base)

	target := "all projects"
	if project != "" {
		target = fmt.Sprintf("project %s", project)
	}

	if !force {
		fmt.Printf("This will delete cached tickets for %s from %s. Continue? [y/N] ", target, cfg.Storage.Base)
		var answer string
		if _, err := fmt.Scanln(&answer); err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}
		if answer != "y" && answer != "Y" {
			return fmt.Errorf("cancelled")
		}
	}

	removed, err := s.Clear(project)
	if err != nil {
		return err
	}

	logStep(cfg.Verbose, "removed %d cached file(s) from %s", removed, cfg.Storage.Base)
	fmt.Printf("Cleared cache for %s (%d file(s) removed)\n", target, removed)
	return nil
}
