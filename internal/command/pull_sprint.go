package command

import (
	"fmt"

	"xynapse/internal/client"
	"xynapse/internal/config"
	"xynapse/internal/storage"
)

func PullSprint(cfg *config.Config, types []string) error {
	logStep(cfg.Verbose, "creating Jira client for %s", cfg.Jira.URL)

	c := client.NewJiraClient(cfg.Jira.URL, cfg.Jira.Email, cfg.Jira.APIToken, cfg.Jira.TimeoutSeconds)

	var sprintID int
	var sprintName string
	if cfg.Defaults.BoardID != "" {
		logStep(cfg.Verbose, "fetching active sprint for board %s", cfg.Defaults.BoardID)
		sprint, err := c.FetchActiveSprint(cfg.Defaults.BoardID)
		if err != nil {
			return err
		}
		sprintID = sprint.ID
		sprintName = sprint.Name
		logStep(cfg.Verbose, "active sprint found: %s (id=%d)", sprintName, sprintID)
	} else {
		logStep(cfg.Verbose, "no board_id configured, using openSprints() JQL")
	}

	logStep(cfg.Verbose, "querying current sprint issues for project %s (assignee=currentUser(), types=%v)", cfg.Defaults.Project, types)
	tickets, err := c.FetchSprintTickets(cfg.Defaults.Project, sprintID, types, cfg.AcceptanceCriteriaField())
	if err != nil {
		return err
	}
	logStep(cfg.Verbose, "fetched %d sprint ticket(s)", len(tickets))

	s := storage.NewStorage(cfg.Storage.Base)
	logStep(cfg.Verbose, "writing %d ticket(s) and sprint manifest to %s", len(tickets), cfg.Storage.Base)
	if err := s.WriteSprintManifest(cfg.Defaults.Project, sprintID, sprintName, tickets); err != nil {
		return err
	}

	fmt.Printf("Pulled %d tickets from current sprint for %s\n", len(tickets), cfg.Defaults.Project)
	if sprintID > 0 {
		fmt.Printf("Sprint: %s (id=%d)\n", sprintName, sprintID)
	}
	return nil
}
