package command

import (
	"fmt"

	"xynapse/internal/client"
	"xynapse/internal/config"
	"xynapse/internal/storage"
)

func PullTicket(cfg *config.Config, ticketRef string) error {
	project, number, err := ParseTicketRef(ticketRef, cfg.Defaults.Project)
	if err != nil {
		return err
	}

	logStep(cfg.Verbose, "creating Jira client for %s", cfg.Jira.URL)
	c := client.NewJiraClient(cfg.Jira.URL, cfg.Jira.Email, cfg.Jira.APIToken, cfg.Jira.TimeoutSeconds)

	logStep(cfg.Verbose, "fetching ticket %s-%s from Jira", project, number)
	ticket, err := c.FetchTicket(project, number)
	if err != nil {
		return err
	}
	logStep(cfg.Verbose, "ticket %s fetched (status=%s, assignee=%s)", ticket.Key, ticket.Status, ticket.Assignee)

	s := storage.NewStorage(cfg.Storage.Base)
	logStep(cfg.Verbose, "writing ticket to %s", cfg.Storage.Base)
	if err := s.WriteTicket(ticket); err != nil {
		return err
	}

	fmt.Printf("Pulled %s (%s) to %s\n", ticket.Key, ticket.Status, cfg.Storage.Base)
	return nil
}
