package command

import (
	"strings"

	"xynapse/internal/config"
	"xynapse/internal/git"
	"xynapse/internal/models"
)

// branchDefaultTemplate is used when no branch template is configured for the
// ticket's type.
const branchDefaultTemplate = "feature-v5/{Key}"

// expandBranch expands the branch template for a ticket. An override from
// prepare --template wins when set; otherwise the per-type template resolved
// from cfg.Git is used, falling back to branchDefaultTemplate.
func expandBranch(cfg *config.Config, ticket *models.Ticket, override string) (string, error) {
	tmpl := override
	if tmpl == "" {
		tmpl = git.ResolveTemplate(cfg.Git.BranchTemplate, cfg.Git.BranchTemplates, ticket.Type)
	}
	if tmpl == "" {
		tmpl = branchDefaultTemplate
	}
	return git.ExpandTemplate(tmpl, git.TemplateVars{
		Key:     ticket.Key,
		Project: ticket.Project,
		Number:  ticketNumber(ticket.Key),
		Board:   cfg.Defaults.BoardID,
		Summary: ticket.Summary,
	})
}

// ticketNumber returns the numeric part of a ticket key ("PROJ-1" -> "1").
func ticketNumber(key string) string {
	if i := strings.LastIndex(key, "-"); i >= 0 {
		return key[i+1:]
	}
	return key
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}