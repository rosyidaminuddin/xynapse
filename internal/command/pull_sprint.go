package command

import (
	"fmt"

	"xynapse/internal/client"
	"xynapse/internal/config"
	"xynapse/internal/storage"
)

func PullSprint(cfg *config.Config) error {
	c := client.NewJiraClient(cfg.Jira.URL, cfg.Jira.Email, cfg.Jira.APIToken, cfg.Jira.TimeoutSeconds)

	tickets, err := c.FetchSprintTickets(cfg.Defaults.Project)
	if err != nil {
		return err
	}

	s := storage.NewStorage(cfg.Storage.Base)
	if err := s.WriteSprintManifest(cfg.Defaults.Project, tickets); err != nil {
		return err
	}

	fmt.Printf("Pulled %d tickets from current sprint for %s\n", len(tickets), cfg.Defaults.Project)
	return nil
}
