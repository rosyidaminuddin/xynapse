package command

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
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

// SprintTicket annotates a ticket with whether it has a saved implementation
// plan, for sprint listing output.
type SprintTicket struct {
	*models.Ticket
	Plan bool `json:"plan" yaml:"plan"`
}

func printSprintTickets(format string, tickets []SprintTicket) error {
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
		printSprintTable(tickets)
	}
	return nil
}

func printSprintTable(tickets []SprintTicket) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "KEY\tSTATUS\tPLAN\tTYPE\tASSIGNEE\tSUMMARY")
	for _, t := range tickets {
		plan := "no"
		if t.Plan {
			plan = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", t.Key, t.Status, plan, t.Type, t.Assignee, t.Summary)
	}
	w.Flush()
}

// RenderMD writes markdown to w, styled for the terminal via glamour (the
// rendering engine behind glow). When w is not a terminal — piped output,
// redirected files, test buffers — the raw markdown is passed through
// unchanged so ANSI escapes never leak into pipelines.
func RenderMD(w io.Writer, md string) error {
	f, ok := w.(*os.File)
	if !ok || !isTerminal(f) {
		_, err := io.WriteString(w, md)
		return err
	}

	width := 80
	if tw, _, err := term.GetSize(int(f.Fd())); err == nil && tw > 0 {
		width = tw
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return err
	}
	out, err := r.Render(md)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, out)
	return err
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
