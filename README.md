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

# Read a ticket from local YAML (no server call)
./bin/xynapse get-ticket 123

# List all tickets from the active sprint from local YAML
./bin/xynapse get-sprint
```

All commands accept `-v` (or `--verbose`) to log every step to stderr:

```sh
./bin/xynapse -v pull-ticket 123
```

Fetched tickets are stored under `storage/<PROJECT>/<KEY>.yml`, with sprint ticket lists tracked in `storage/<PROJECT>/sprints/current.yml`.

## Help

```sh
./bin/xynapse help
```
