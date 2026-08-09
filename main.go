package main

import (
	"fmt"
	"os"

	"xynapse/internal/command"
	"xynapse/internal/config"
)

func main() {
	cfg, err := config.Load("config/config.yaml", ".env")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	args := os.Args[1:]
	var verbose bool
	var positional []string
	for _, a := range args {
		switch a {
		case "-v", "--verbose":
			verbose = true
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
		err = command.PullSprint(cfg)
	case "get-ticket":
		if len(positional) < 2 {
			fmt.Fprintln(os.Stderr, "usage: xynapse get-ticket <ticket-number>")
			os.Exit(1)
		}
		err = command.GetTicket(cfg, positional[1])
	case "get-sprint":
		err = command.GetSprint(cfg)
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

func usage() {
	fmt.Println(`xynapse - manage Jira tickets locally

usage:
  xynapse [-v] pull-ticket <ticket-number>   fetch a ticket from Jira and write it as yaml
  xynapse [-v] pull-sprint                    fetch all tickets in the current sprint for the current user
  xynapse [-v] get-ticket <ticket-number>     read a ticket from local yaml
  xynapse [-v] get-sprint                     list all tickets from the active sprint

flags:
  -v, --verbose   log every step to stderr`)
}
