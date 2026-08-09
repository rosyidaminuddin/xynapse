package command

import (
	"fmt"
	"os"
	"strings"

	"xynapse/internal/config"
	"xynapse/internal/git"
	"xynapse/internal/models"
	"xynapse/internal/storage"
)

// Finalize commits the working tree on the current branch, pushes it to
// origin, and — when createPR is set and a base branch is given — opens a pull
// request via gh. The ticket's plan status is marked done on success. It
// refuses to run on the base branch (when one is supplied).
func Finalize(cfg *config.Config, ticketRef, dir, base, message string, createPR bool) error {
	s := storage.NewStorage(cfg.Storage.Base)
	ticket, err := resolveTicket(cfg, s, ticketRef)
	if err != nil {
		return err
	}

	g := git.New(dir, "")
	branch, err := g.CurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to read current branch: %w", err)
	}
	logStep(cfg.Verbose, "on branch %s", branch)

	if base != "" && branch == base {
		return fmt.Errorf("refusing to finalize on base branch %q; check out a feature branch first", base)
	}

	dirty, err := g.IsDirty()
	if err != nil {
		return fmt.Errorf("failed to inspect working tree: %w", err)
	}

	if dirty {
		if message == "" {
			message = fmt.Sprintf("%s: %s", ticket.Key, ticket.Summary)
		}
		logStep(cfg.Verbose, "staging all changes and committing %q", message)
		if err := g.AddAll(); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}
		if err := g.Commit(message); err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}
		fmt.Printf("Committed %q\n", message)
	} else {
		fmt.Println("No changes to commit (working tree is clean)")
	}

	logStep(cfg.Verbose, "pushing %s to origin", branch)
	if err := g.Push(branch); err != nil {
		return fmt.Errorf("failed to push %s: %w", branch, err)
	}
	fmt.Printf("Pushed %s to origin/%s\n", branch, branch)

	if createPR {
		if base == "" {
			return fmt.Errorf("--pr requires the base branch; pass --base <branch>")
		}
		title := message
		if title == "" {
			title = fmt.Sprintf("%s: %s", ticket.Key, ticket.Summary)
		}
		logStep(cfg.Verbose, "creating pull request via gh (base=%s)", base)
		url, err := git.CreatePR("gh", dir, git.PROptions{
			Base:  base,
			Head:  branch,
			Title: title,
			Body:  prBody(cfg, ticket),
		})
		if err != nil {
			return err
		}
		fmt.Printf("Pull request created: %s\n", url)
	}

	if err := s.SetPlanStatus(ticket.Project, ticket.Key, storage.PlanStatusDone); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not mark plan done: %v\n", err)
	}
	return nil
}

// prBody builds the pull request description: a "Closes <KEY>" trailer plus a
// link back to the ticket on Jira.
func prBody(cfg *config.Config, t *models.Ticket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Closes %s", t.Key)
	if cfg.Jira.URL != "" {
		fmt.Fprintf(&b, "\n\n%s", strings.TrimRight(cfg.Jira.URL, "/")+"/browse/"+t.Key)
	}
	return b.String()
}
