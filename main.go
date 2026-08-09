package main

import (
	"fmt"
	"os"
	"strings"

	"xynapse/internal/command"
	"xynapse/internal/config"
)

func main() {
	cfg, err := config.Load("config/config.yaml", ".env")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var verbose bool
	var types []string
	var positional []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-v" || a == "--verbose":
			verbose = true
		case a == "-t" || a == "--type":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: flag -t/--type requires a value")
				os.Exit(1)
			}
			i++
			types = splitTypes(args[i])
		case strings.HasPrefix(a, "--type="):
			types = splitTypes(strings.TrimPrefix(a, "--type="))
		case strings.HasPrefix(a, "-t="):
			types = splitTypes(strings.TrimPrefix(a, "-t="))
		default:
			positional = append(positional, a)
		}
	}
	cfg.Verbose = verbose

	if len(positional) == 0 {
		usage()
		os.Exit(1)
	}

	switch positional[0] {
	case "pull-ticket":
		if len(positional) < 2 {
			fmt.Fprintln(os.Stderr, "usage: xynapse pull-ticket <ticket-number>")
			os.Exit(1)
		}
		err = command.PullTicket(cfg, positional[1])
	case "pull-sprint":
		err = command.PullSprint(cfg, types)
	case "get-ticket":
		if len(positional) < 2 {
			fmt.Fprintln(os.Stderr, "usage: xynapse get-ticket <ticket-number>")
			os.Exit(1)
		}
		err = command.GetTicket(cfg, positional[1])
	case "get-sprint":
		err = command.GetSprint(cfg, types)
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", positional[0])
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// splitTypes splits a comma-separated type list into trimmed non-empty values.
func splitTypes(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func usage() {
	fmt.Println(`xynapse - manage Jira tickets locally

usage:
  xynapse [-v] pull-ticket <ticket-number>   fetch a ticket from Jira and write it as yaml
  xynapse [-v] [-t <type>] pull-sprint        fetch all tickets in the current sprint for the current user
  xynapse [-v] get-ticket <ticket-number>     read a ticket from local yaml
  xynapse [-v] [-t <type>] get-sprint         list all tickets from the active sprint

flags:
  -v, --verbose           log every step to stderr
  -t, --type <types>      comma-separated issue types to filter by (e.g. Story,Bug,Epic)`)
}
