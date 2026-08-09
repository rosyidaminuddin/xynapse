package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xynapse/internal/config"
	"xynapse/internal/models"
	"xynapse/internal/opencode"
	"xynapse/internal/storage"
)

// resolveTicket reads a ticket from the local cache, refreshing from Jira
// when the cached copy is stale (per expiration.hours).
func resolveTicket(cfg *config.Config, s *storage.Storage, ticketRef string) (*models.Ticket, error) {
	project, number, err := ParseTicketRef(ticketRef, cfg.Defaults.Project)
	if err != nil {
		return nil, err
	}

	logStep(cfg.Verbose, "reading local ticket %s-%s from %s", project, number, cfg.Storage.Base)
	ticket, err := s.ReadTicket(project, number)
	if err != nil {
		return nil, err
	}

	if expired := cfg.Expiration.Duration(); expired > 0 && time.Since(ticket.FetchedAt) > expired {
		logStep(cfg.Verbose, "cached ticket is stale (older than %s), refreshing from Jira", expired)
		if err := PullTicket(cfg, ticketRef); err != nil {
			return nil, err
		}
		ticket, err = s.ReadTicket(project, number)
		if err != nil {
			return nil, err
		}
	}
	return ticket, nil
}

// ticketDossier renders a compact markdown summary of a ticket to feed an
// opencode skill.
func ticketDossier(t *models.Ticket) string {
	desc := strings.TrimSpace(t.DescriptionText)
	if len(desc) > 4000 {
		desc = desc[:4000] + "..."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Key: %s\n", t.Key)
	fmt.Fprintf(&b, "Type: %s\n", t.Type)
	fmt.Fprintf(&b, "Status: %s\n", t.Status)
	fmt.Fprintf(&b, "Assignee: %s\n", t.Assignee)
	fmt.Fprintf(&b, "Summary: %s\n", t.Summary)
	if desc != "" {
		fmt.Fprintf(&b, "\nDescription:\n%s\n", desc)
	}
	return b.String()
}

// Plan runs the analyze-ticket skill via opencode and saves the resulting
// implementation plan to <storage>/plans/<KEY>.md.
func Plan(cfg *config.Config, ticketRef, dir, model string) error {
	s := storage.NewStorage(cfg.Storage.Base)
	ticket, err := resolveTicket(cfg, s, ticketRef)
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf("Use the analyze-ticket skill. Analyze this ticket and produce a step-by-step implementation plan.\n\nTicket:\n%s", ticketDossier(ticket))

	logStep(cfg.Verbose, "running opencode analyze-ticket skill (dir=%s)", dir)
	out, err := opencode.Run(opencode.Options{
		Bin:         cfg.Opencode.Bin,
		Dir:         dir,
		Model:       model,
		AutoApprove: false,
		Prompt:      prompt,
		Stream:      os.Stderr,
	})
	if err != nil {
		return err
	}
	planText := opencode.ExtractText(out)
	if planText == "" {
		return fmt.Errorf("opencode returned an empty plan for %s", ticket.Key)
	}

	planPath := s.GetPlanPath(ticket.Project, ticket.Key)
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		return fmt.Errorf("failed to create plans dir: %w", err)
	}
	if err := os.WriteFile(planPath, []byte(planText), 0o644); err != nil {
		return fmt.Errorf("failed to write plan %s: %w", planPath, err)
	}

	if err := RenderMD(os.Stdout, planText); err != nil {
		return fmt.Errorf("failed to render plan: %w", err)
	}
	fmt.Printf("\nPlan saved to %s. Review it, then run: xynapse implement %s\n", planPath, ticket.Key)
	return nil
}
