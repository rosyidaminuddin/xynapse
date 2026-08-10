package command

import (
	"fmt"
	"strings"

	"xynapse/internal/client"
	"xynapse/internal/config"
	"xynapse/internal/storage"
)

// Transition moves a Jira ticket to a new status. With no statusName it lists
// the available transitions. The target status is matched case-insensitively
// against the transition's target status; transitionID forces a specific
// transition. On success the local ticket cache is refreshed.
func Transition(cfg *config.Config, ticketRef, statusName, transitionID string) error {
	project, number, err := ParseTicketRef(ticketRef, cfg.Defaults.Project)
	if err != nil {
		return err
	}

	c := client.NewJiraClient(cfg.Jira.URL, cfg.Jira.Email, cfg.Jira.APIToken, cfg.Jira.TimeoutSeconds)

	logStep(cfg.Verbose, "fetching available transitions for %s-%s", project, number)
	transitions, err := c.FetchTransitions(project, number)
	if err != nil {
		return err
	}
	if len(transitions) == 0 {
		return fmt.Errorf("no transitions available for %s-%s", project, number)
	}

	if transitionID != "" {
		for _, t := range transitions {
			if t.ID == transitionID {
				return executeTransition(cfg, c, project, number, t)
			}
		}
		return fmt.Errorf("transition id %s not available for %s-%s", transitionID, project, number)
	}

	if statusName == "" {
		printTransitions(project, number, transitions)
		return nil
	}

	want := strings.ToLower(strings.TrimSpace(statusName))
	var matches []client.Transition
	for _, t := range transitions {
		if strings.ToLower(t.To) == want || strings.ToLower(t.Name) == want {
			matches = append(matches, t)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("no transition to %q for %s-%s; available: %s",
			statusName, project, number, strings.Join(transitionNames(transitions), ", "))
	}
	if len(matches) > 1 {
		var ids []string
		for _, t := range matches {
			ids = append(ids, t.ID)
		}
		return fmt.Errorf("multiple transitions to %q for %s-%s; use --id to pick one: %s",
			statusName, project, number, strings.Join(ids, ", "))
	}
	return executeTransition(cfg, c, project, number, matches[0])
}

func executeTransition(cfg *config.Config, c *client.JiraClient, project, number string, t client.Transition) error {
	logStep(cfg.Verbose, "transitioning %s-%s via transition %s (%s)", project, number, t.ID, t.Name)
	if err := c.TransitionTicket(project, number, t.ID); err != nil {
		return err
	}
	fmt.Printf("Transitioned %s-%s to %s\n", project, number, t.To)

	logStep(cfg.Verbose, "refreshing cached ticket %s-%s", project, number)
	if err := refreshTicket(cfg, c, project, number); err != nil {
		fmt.Printf("warning: could not refresh cached ticket: %v\n", err)
	}
	return nil
}

// refreshTicket silently re-fetches a ticket from Jira and rewrites the local
// cache so the status and updated timestamp stay current.
func refreshTicket(cfg *config.Config, c *client.JiraClient, project, number string) error {
	ticket, err := c.FetchTicket(project, number)
	if err != nil {
		return err
	}
	return storage.NewStorage(cfg.Storage.Base).WriteTicket(ticket)
}

func printTransitions(project, number string, transitions []client.Transition) {
	fmt.Printf("Available transitions for %s-%s:\n", project, number)
	fmt.Printf("  %-4s %-20s %s\n", "ID", "Transition", "To")
	for _, t := range transitions {
		fmt.Printf("  %-4s %-20s %s\n", t.ID, t.Name, t.To)
	}
}

func transitionNames(transitions []client.Transition) []string {
	names := make([]string, 0, len(transitions))
	for _, t := range transitions {
		names = append(names, t.To)
	}
	return names
}
