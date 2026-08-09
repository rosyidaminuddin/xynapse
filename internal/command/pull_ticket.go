package command

import (
	"fmt"

	"xynapse/internal/client"
	"xynapse/internal/config"
	"xynapse/internal/storage"
)

func PullTicket(cfg *config.Config, ticketNum string) error {
	logStep(cfg.Verbose, "creating Jira client for %s", cfg.Jira.URL)

	c := client.NewJiraClient(cfg.Jira.URL, cfg.Jira.Email, cfg.Jira.APIToken, cfg.Jira.TimeoutSeconds)
	c.SetVerbose(cfg.Verbose)

	logStep(cfg.Verbose, "fetching ticket %s-%s from Jira", cfg.Defaults.Project, ticketNum)
	ticket, err := c.FetchTicket(cfg.Defaults.Project, ticketNum)
	if err != nil {
		return err
	}
	logStep(cfg.Verbose, "ticket %s fetched (status=%s, assignee=%s)", ticket.Key, ticket.Status, ticket.Assignee)

	s := storage.NewStorage(cfg.Storage.Base)
	logStep(cfg.Verbose, "writing ticket to %s", cfg.Storage.Base)
	if err := s.WriteTicket(ticket); err != nil {
		return err
	}

	fmt.Printf("Pulled %s (%s) to %s/%s.yml\n", ticket.Key, ticket.Status, cfg.Storage.Base, ticket.Project)
	return nil
}
