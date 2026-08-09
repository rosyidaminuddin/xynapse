package command

import (
	"fmt"

	"xynapse/internal/client"
	"xynapse/internal/config"
	"xynapse/internal/storage"
)

func PullSprint(cfg *config.Config) error {
	logStep(cfg.Verbose, "creating Jira client for %s", cfg.Jira.URL)

	c := client.NewJiraClient(cfg.Jira.URL, cfg.Jira.Email, cfg.Jira.APIToken, cfg.Jira.TimeoutSeconds)
	c.SetVerbose(cfg.Verbose)

	logStep(cfg.Verbose, "querying current sprint issues for project %s (assignee=currentUser())", cfg.Defaults.Project)
	tickets, err := c.FetchSprintTickets(cfg.Defaults.Project)
	if err != nil {
		return err
	}
	logStep(cfg.Verbose, "fetched %d sprint ticket(s)", len(tickets))

	s := storage.NewStorage(cfg.Storage.Base)
	logStep(cfg.Verbose, "writing %d ticket(s) and sprint manifest to %s", len(tickets), cfg.Storage.Base)
	if err := s.WriteSprintManifest(cfg.Defaults.Project, tickets); err != nil {
		return err
	}

	fmt.Printf("Pulled %d tickets from current sprint for %s\n", len(tickets), cfg.Defaults.Project)
	return nil
}
