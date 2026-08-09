package command

import (
	"fmt"

	"xynapse/internal/client"
	"xynapse/internal/config"
	"xynapse/internal/storage"
)

func PullTicket(cfg *config.Config, ticketNum string) error {
	c := client.NewJiraClient(cfg.Jira.URL, cfg.Jira.Email, cfg.Jira.APIToken, cfg.Jira.TimeoutSeconds)

	ticket, err := c.FetchTicket(cfg.Defaults.Project, ticketNum)
	if err != nil {
		return err
	}

	s := storage.NewStorage(cfg.Storage.Base)
	if err := s.WriteTicket(ticket); err != nil {
		return err
	}

	fmt.Printf("Pulled %s (%s) to %s/%s.yml\n", ticket.Key, ticket.Status, cfg.Storage.Base, ticket.Project)
	return nil
}
