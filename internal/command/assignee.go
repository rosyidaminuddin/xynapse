package command

import (
	"fmt"
	"strings"

	"xynapse/internal/client"
	"xynapse/internal/config"
)

// Assignee shows or updates a ticket's assignee. With no user it prints the
// current assignee fetched live from Jira. A user is resolved by exact display
// name or email via user search, or passed straight through when it looks like
// an account ID. The keywords "unassigned", "none", and "-" clear the
// assignee. On success the local ticket cache is refreshed.
func Assignee(cfg *config.Config, ticketRef, user string) error {
	project, number, err := ParseTicketRef(ticketRef, cfg.Defaults.Project)
	if err != nil {
		return err
	}

	c := client.NewJiraClient(cfg.Jira.URL, cfg.Jira.Email, cfg.Jira.APIToken, cfg.Jira.TimeoutSeconds)

	if strings.TrimSpace(user) == "" {
		return showAssignee(cfg, c, project, number)
	}

	unassign := isUnassignKeyword(user)
	var accountID string
	var name string

	switch {
	case unassign:
		accountID = ""
	case isAccountID(user):
		accountID = user
	default:
		u, err := resolveUser(c, user)
		if err != nil {
			return err
		}
		accountID = u.AccountID
		name = u.DisplayName
	}

	logStep(cfg.Verbose, "updating assignee for %s-%s", project, number)
	if err := c.SetAssignee(project, number, accountID, unassign); err != nil {
		return err
	}

	if unassign {
		fmt.Printf("Unassigned %s-%s\n", project, number)
	} else {
		if name == "" {
			name = accountID
		}
		fmt.Printf("Assigned %s-%s to %s\n", project, number, name)
	}

	logStep(cfg.Verbose, "refreshing cached ticket %s-%s", project, number)
	if err := refreshTicket(cfg, c, project, number); err != nil {
		fmt.Printf("warning: could not refresh cached ticket: %v\n", err)
	}
	return nil
}

// showAssignee fetches the ticket live and prints its current assignee.
func showAssignee(cfg *config.Config, c *client.JiraClient, project, number string) error {
	logStep(cfg.Verbose, "fetching ticket %s-%s", project, number)
	ticket, err := c.FetchTicket(project, number, cfg.AcceptanceCriteriaField())
	if err != nil {
		return err
	}
	if ticket.Assignee == "" || ticket.Assignee == "Unassigned" {
		fmt.Printf("%s-%s is unassigned\n", project, number)
		return nil
	}
	fmt.Printf("%s-%s is assigned to %s\n", project, number, ticket.Assignee)
	return nil
}

// resolveUser finds a Jira user by display name or email. An exact
// case-insensitive match on either wins; otherwise a single search result is
// used and multiple results produce an error listing candidates to refine by.
func resolveUser(c *client.JiraClient, user string) (*client.JiraUser, error) {
	want := strings.ToLower(strings.TrimSpace(user))

	users, err := c.SearchUsers(user)
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("no Jira user matches %q", user)
	}

	var exact []client.JiraUser
	for _, u := range users {
		if strings.ToLower(u.DisplayName) == want || strings.ToLower(u.Email) == want {
			exact = append(exact, u)
		}
	}
	if len(exact) == 1 {
		return &exact[0], nil
	}
	if len(exact) > 1 {
		return nil, fmt.Errorf("multiple Jira users match %q; refine your query:\n%s",
			user, formatUserCandidates(exact, user))
	}

	if len(users) == 1 {
		return &users[0], nil
	}
	return nil, fmt.Errorf("ambiguous Jira user %q; use a display name or full email:\n%s",
		user, formatUserCandidates(users, user))
}

// maxShownCandidates is the number of matching users printed before eliding
// the rest with a summary line.
const maxShownCandidates = 3

// formatUserCandidates renders a numbered, one-per-line listing of matching
// users, capped at maxShownCandidates. Extra matches are summarized so the
// caller can tell the query was too broad.
func formatUserCandidates(users []client.JiraUser, query string) string {
	shown := users
	if len(users) > maxShownCandidates {
		shown = users[:maxShownCandidates]
	}

	var sb strings.Builder
	for i, u := range shown {
		label := u.DisplayName
		if u.Email != "" {
			label += " <" + u.Email + ">"
		}
		if u.AccountID != "" {
			label += " (" + u.AccountID + ")"
		}
		fmt.Fprintf(&sb, "  %d. %s\n", i+1, label)
	}
	if len(users) > maxShownCandidates {
		fmt.Fprintf(&sb, "  ... and %d more match %q\n", len(users)-maxShownCandidates, query)
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

func isUnassignKeyword(user string) bool {
	switch strings.ToLower(strings.TrimSpace(user)) {
	case "unassigned", "none", "-":
		return true
	}
	return false
}

// isAccountID reports whether s looks like a Jira account ID (a long
// alphanumeric string, e.g. 5b10a2844c20165700ede21g).
func isAccountID(s string) bool {
	if len(s) < 16 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
