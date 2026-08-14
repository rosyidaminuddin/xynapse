package command

import (
	"fmt"
	"os"
	"text/tabwriter"

	"xynapse/internal/config"
	"xynapse/internal/models"
	"xynapse/internal/storage"
)

func GetTicket(cfg *config.Config, ticketRef, format string) error {
	s := storage.NewStorage(cfg.Storage.Base)
	ticket, err := resolveTicket(cfg, s, ticketRef)
	if err != nil {
		return err
	}

	if format == "" {
		format = cfg.Defaults.OutputFormat
	}

	if format == "json" || format == "yaml" {
		return printTickets(format, []*models.Ticket{ticket})
	}

	snap := collectGitSnapshot(cfg)
	plan := planStatus(s, cfg.Defaults.Project, ticket.Key, ticket.UpdatedAt)
	derived, local := sprintBranch(cfg, ticket, snap)
	finalized := finalizedStatus(snap, ticket.Key)
	pr := prStatus(snap, effectiveBranch(local, derived))
	target := prTarget(snap, effectiveBranch(local, derived))

	printTicketTable([]models.Ticket{{Key: ticket.Key, Status: ticket.Status, Type: ticket.Type, Summary: ticket.Summary, Assignee: ticket.Assignee}}, plan, finalized, pr, target, local, cfg)
	return nil
}

func printTicketTable(tickets []models.Ticket, plan, finalized, pr, target, local string, cfg *config.Config) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "KEY\tSTATUS\tFINALIZED\tPR\tTARGET\tBRANCH\tPLAN\tTYPE\tASSIGNEE\tSUMMARY")
	for _, t := range tickets {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			t.Key, t.Status, finalized, pr, target, local, plan, t.Type, t.Assignee, t.Summary)
	}
	w.Flush()
}