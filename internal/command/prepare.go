package command

import (
	"fmt"

	"xynapse/internal/config"
	"xynapse/internal/git"
	"xynapse/internal/storage"
)

// Prepare creates (or checks out) a feature branch for a ticket, branched
// from a user-supplied base branch. The branch name is produced by expanding
// the configured branch template (default feature-v5/{Key}). It refuses to
// run when the working tree is dirty unless force is set.
func Prepare(cfg *config.Config, ticketRef, dir, base, template string, force bool) error {
	s := storage.NewStorage(cfg.Storage.Base)
	ticket, err := resolveTicket(cfg, s, ticketRef)
	if err != nil {
		return err
	}

	tmpl := template
	if tmpl == "" {
		tmpl = git.ResolveTemplate(cfg.Git.BranchTemplate, cfg.Git.BranchTemplates, ticket.Type)
	}
	if tmpl == "" {
		tmpl = "feature-v5/{Key}"
	}

	_, number, err := ParseTicketRef(ticketRef, cfg.Defaults.Project)
	if err != nil {
		return err
	}
	branch, err := git.ExpandTemplate(tmpl, git.TemplateVars{
		Key:     ticket.Key,
		Project: ticket.Project,
		Number:  number,
		Board:   cfg.Defaults.BoardID,
		Summary: ticket.Summary,
	})
	if err != nil {
		return err
	}

	g := git.New(dir, "")
	if dir == "" {
		logStep(cfg.Verbose, "using current working directory for git operations")
	} else {
		logStep(cfg.Verbose, "using git directory %s", dir)
	}

	dirty, err := g.IsDirty()
	if err != nil {
		return fmt.Errorf("failed to inspect working tree: %w", err)
	}
	if dirty && !force {
		return fmt.Errorf("working tree is dirty; commit or stash your changes first, or pass --force to branch anyway")
	}

	if g.BranchExists(branch) {
		logStep(cfg.Verbose, "branch %s already exists, checking it out", branch)
		if err := g.Checkout(branch); err != nil {
			return fmt.Errorf("failed to check out %s: %w", branch, err)
		}
		fmt.Printf("Already on branch %s\n", branch)
	} else {
		logStep(cfg.Verbose, "checking out base branch %s", base)
		if err := g.Checkout(base); err != nil {
			return fmt.Errorf("failed to check out base branch %s: %w", base, err)
		}
		if err := g.CreateBranch(branch); err != nil {
			return fmt.Errorf("failed to create branch %s: %w", branch, err)
		}
		fmt.Printf("Created branch %s from %s\n", branch, base)
	}

	fmt.Printf("Run `xynapse implement %s` to execute the plan on this branch\n", ticket.Key)
	return nil
}
