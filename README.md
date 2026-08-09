# xynapse

A CLI to manage Jira tickets locally. Tickets are fetched from the Jira REST API and cached as YAML files, so `get` commands read from disk without hitting the server.

## Requirements

- Go 1.26+
- A Jira Cloud account with API token access

## Install

```sh
./install.sh              # install to ~/.local/bin
PREFIX=/usr/local ./install.sh   # install to /usr/local/bin (requires sudo)
```

The script builds the binary into `bin/`, copies it to a directory on `PATH` (default `~/.local/bin`), and installs the example config to `~/.config/xynapse/config.yaml` on first run. Add to your shell rc if `~/.local/bin` is not on `PATH`:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

## Configuration

Config is always loaded from `~/.config/xynapse/` — never from project files. Edit `~/.config/xynapse/config.yaml`:

```yaml
jira:
  url: "https://yourcompany.atlassian.net"
  email: "you@example.com"
  api_token: ""          # or set via JIRA_API_TOKEN / .env
  timeout_seconds: 15

defaults:
  project: "MERADIO"
  board_id: "561"        # optional: used by pull-sprint to resolve the active sprint
  output_format: "table" # table, json, or yaml

storage:
  base: "storage"

expiration:
  hours: 24              # get commands auto-refresh cached data older than this; 0 disables
```

Credentials are resolved from (highest priority first):

1. Environment variables `JIRA_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`
2. A `.env` file in `~/.config/xynapse/` (same variable names)
3. Values in `~/.config/xynapse/config.yaml`

> Never commit credentials. `~/.config/xynapse/` is per-user and not shared.

Set `XYNAPSE_CONFIG_DIR` to override the config directory and `XYNAPSE_STORAGE` to override the storage directory.

## Build

```sh
go build -o bin/xynapse .
```

## Test

```sh
go test ./...
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

### Ticket references

`pull-ticket` and `get-ticket` accept a bare number, a full key, or a browse URL:

```sh
./bin/xynapse get-ticket 123
./bin/xynapse get-ticket MERADIO-123
./bin/xynapse get-ticket https://yourcompany.atlassian.net/browse/MERADIO-123
```

### Flags

- `-v`, `--verbose` — log every step to stderr
- `-p`, `--project <KEY>` — override the default project for this invocation
- `-t`, `--type <types>` — comma-separated issue types (e.g. `Story,Bug,Epic`). For `pull-sprint` it is applied as a JQL `issuetype in (...)` filter on the server; for `get-sprint` it filters the locally cached tickets.
- `-o`, `--output <format>` — output format for `get-ticket`: `table`, `json`, or `yaml` (overrides config)

### Cache expiration

When cached data is older than `expiration.hours`, `get-ticket` and `get-sprint` automatically re-pull from Jira before printing. Set `expiration.hours` to `0` to disable.

### Storage layout

Fetched tickets are stored under `storage/<PROJECT>/<KEY>.yml`, with sprint ticket lists tracked in `storage/<PROJECT>/sprints/current.yml`. When `board_id` is configured, `pull-sprint` looks up the active sprint via the Jira Agile API and stores its `sprint_id` and `sprint_name` in the manifest. Each ticket keeps both the raw ADF description JSON (`description`) and a flattened plain-text copy (`description_text`).

## Help

```sh
./bin/xynapse --help
```
