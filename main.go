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

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "pull-ticket":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: xynapse pull-ticket <ticket-number>")
			os.Exit(1)
		}
		err = command.PullTicket(cfg, os.Args[2])
	case "pull-sprint":
		err = command.PullSprint(cfg)
	case "get-ticket":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: xynapse get-ticket <ticket-number>")
			os.Exit(1)
		}
		err = command.GetTicket(cfg, os.Args[2])
	case "get-sprint":
		err = command.GetSprint(cfg)
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
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
  xynapse pull-ticket <ticket-number>   fetch a ticket from Jira and write it as yaml
  xynapse pull-sprint                    fetch all tickets in the current sprint for the current user
  xynapse get-ticket <ticket-number>     read a ticket from local yaml
  xynapse get-sprint                     list all tickets from the active sprint`)
}
