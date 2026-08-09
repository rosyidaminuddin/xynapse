package command

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"gopkg.in/yaml.v3"

	"xynapse/internal/models"
)

func printTickets(format string, tickets []*models.Ticket) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(tickets, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(tickets)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Print(string(data))
	default:
		printTable(tickets)
	}
	return nil
}

func printTable(tickets []*models.Ticket) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "KEY\tSTATUS\tASSIGNEE\tSUMMARY")
	for _, t := range tickets {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", t.Key, t.Status, t.Assignee, t.Summary)
	}
	w.Flush()
}
