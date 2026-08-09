# xynapse

A CLI to manage Jira tickets locally. Tickets are fetched from the Jira REST API and cached as YAML files, so `get` commands read from disk without hitting the server.

## Requirements

- Go 1.26+
- A Jira Cloud account with API token access

## Configuration

Copy the config and set your credentials. Edit `config/config.yaml`:

```yaml
jira:
  url: "https://yourcompany.atlassian.net"
  email: "you@example.com"
  api_token: ""          # or set via JIRA_API_TOKEN / .env
  timeout_seconds: 15

defaults:
  project: "MERADIO"
  output_format: "table" # table, json, or yaml

storage:
  base: "storage"
```

Credentials are resolved from (highest priority first):

1. Environment variables `JIRA_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`
2. A `.env` file in the project root (same variable names)
3. Values in `config/config.yaml`

> `.env` is gitignored — never commit it.

## Build

```sh
go build -o bin/xynapse .
```

## Run

```sh
# Fetch a ticket from Jira and write it as a YAML file
./bin/xynapse pull-ticket 123

# Fetch all tickets in the current sprint for the current user
./bin/xynapse pull-sprint

# Fetch only Story and Bug tickets from the current sprint
./bin/xynapse pull-sprint -t Story,Bug

# Read a ticket from local YAML (no server call)
./bin/xynapse get-ticket 123

# List all tickets from the active sprint from local YAML
./bin/xynapse get-sprint

# List only Epic tickets from the active sprint
./bin/xynapse get-sprint -t Epic
```

All commands accept `-v` (or `--verbose`) to log every step to stderr:

```sh
./bin/xynapse -v pull-ticket 123
```

The `-t`/`--type` flag takes a comma-separated list of issue types (e.g. `Story,Bug,Epic`). For `pull-sprint` it is applied as a JQL `issuetype in (...)` filter on the server; for `get-sprint` it filters the locally cached tickets.

Fetched tickets are stored under `storage/<PROJECT>/<KEY>.yml`, with sprint ticket lists tracked in `storage/<PROJECT>/sprints/current.yml`. When `board_id` is configured, `pull-sprint` looks up the active sprint via the Jira Agile API and stores its `sprint_id` and `sprint_name` in the manifest.

## Help

```sh
./bin/xynapse help
```
