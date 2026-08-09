# xynapse

A CLI to manage Jira tickets locally. Tickets are fetched from the Jira REST API and cached as YAML files, so `get` commands read from disk without hitting the server.

## Requirements

- Go 1.26+
- A Jira Cloud account with API token access
- [opencode](https://opencode.ai) CLI for the `plan` and `implement` commands
- [gh](https://cli.github.com) CLI (installed and authenticated) for `finalize --pr`

## Install

```sh
./install.sh              # install to ~/.local/bin
PREFIX=/usr/local ./install.sh   # install to /usr/local/bin (requires sudo)
```

The script builds the binary into `bin/`, copies it to a directory on `PATH` (default `~/.local/bin`), installs the example config to `~/.config/xynapse/config.yaml` on first run, and syncs the opencode skills (`.opencode/skills/`) to `~/.config/opencode/skills/`. Add to your shell rc if `~/.local/bin` is not on `PATH`:

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

opencode:
  bin: "opencode"        # path or name of the opencode binary on PATH
  model: ""              # optional provider/model override, e.g. "anthropic/claude-sonnet-4"
  auto_approve: false    # pass --auto to opencode run (implement may edit files without prompting)
  dir: ""                # target repo directory (default: current working directory)

git:
  branch_template: "feature-v5/{Key}"  # branch naming for `prepare`; see placeholders below
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

# List all tickets from the active sprint from local YAML (PLAN: yes/stale/no)
./bin/xynapse get-sprint

# List only Epic tickets from the active sprint
./bin/xynapse get-sprint -t Epic

# List tickets that have no saved plan yet
./bin/xynapse get-sprint --unplanned

# Delete all locally cached tickets and plans (prompts for confirmation)
./bin/xynapse clear-cache

# Delete the cached MERADIO tickets and MERADIO plans without prompting
./bin/xynapse clear-cache -f -p MERADIO

# Analyze a ticket and generate an implementation plan (saved to <storage>/plans/<KEY>.md)
./bin/xynapse plan MERADIO-123

# Plan against a specific repo, or execute a saved plan in it
./bin/xynapse plan MERADIO-123 --dir ~/work/myproject
./bin/xynapse implement MERADIO-123 --dir ~/work/myproject

# Analyze a ticket on an existing feature branch
./bin/xynapse plan MERADIO-123 -b feature-v5/MERADIO-123

# Display a saved plan as styled markdown in the terminal
./bin/xynapse show-plan MERADIO-123

# Show or update a ticket's plan status (not started, in progress, in review, done)
./bin/xynapse status MERADIO-123
./bin/xynapse status MERADIO-123 --set in_review

# Create a feature branch (feature-v5/MERADIO-123) from main
./bin/xynapse prepare MERADIO-123 -b main

# Commit, push, and open a pull request against main
./bin/xynapse finalize MERADIO-123 -b main --pr
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
- `-f`, `--force` — skip the confirmation prompt (`clear-cache`, `implement`) or the dirty-tree check (`prepare`)
- `--dir <repo>` — target repo directory for `plan`/`implement`/`prepare`/`finalize` (defaults to `opencode.dir`, then cwd)
- `--model <provider/model>` — override the opencode model for `plan`/`implement`
- `-b`, `--branch <branch>` — check out this branch in the target repo before `plan` analyzes the ticket
- `--plan <path>` — use a specific plan file for `implement` (default: `<storage>/plans/<KEY>.md`)
- `-b`, `--base <branch>` — base branch: required for `prepare` (branch from); for `finalize` it is the PR target and makes finalize refuse to commit on it
- `--template <tmpl>` — override the branch template for `prepare`
- `--message <msg>` — commit message for `finalize` (default: `"<KEY>: <summary>"`)
- `--pr` — create a pull request with `gh` after `finalize` pushes (requires `--base`)
- `--set <status>` — update a plan's status with `status` (not started, in progress, in review, done)

### Implementation plans

`plan` and `implement` delegate to the [opencode](https://opencode.ai) CLI and two skills shipped in this repo (`.opencode/skills/`, synced to `~/.config/opencode/skills/` by `install.sh`):

- `xynapse plan <ref>` — fetches/refreshes the ticket, runs the `analyze-ticket` skill to produce a step-by-step implementation plan, and saves it to `<storage>/plans/<KEY>.md`.
- `xynapse implement <ref>` — runs the `implement-plan` skill in the target repo to execute the saved plan. The agent leaves changes in the working tree; it does not commit or push.
- `xynapse show-plan <ref>` — displays a ticket's saved plan from `<storage>/plans/<KEY>.md` as styled markdown (works without a cached ticket).
- `xynapse status <ref> [--set <status>]` — show or update a plan's status.

Each plan file carries a lifecycle **status** in its YAML frontmatter: `not started` (set by `plan`), `in progress` (set by `implement`), `in review`, and `done` (set by `finalize`). Update it manually with `xynapse status <ref> --set in_review`. `show-plan` displays the status as a header.

A plan is **stale** when the ticket changed on Jira after it was written. `get-sprint` reflects this in the PLAN column (`yes`/`stale`/`no`), and `show-plan` warns on stderr. `implement` warns and asks for confirmation before running a stale plan; pass `--force` to skip the prompt. Staleness is only checked when the ticket is cached — plan-only workflows keep working offline.

While opencode works, its activity is streamed live to stderr — each tool call (bash command, file edit, read, etc.) is printed as `  [tool] title` the moment it runs — so you can watch what the agent is doing in the terminal. Assistant text is shown once, when the command finishes.

Review the plan before executing. Skills require the opencode CLI to be installed and authenticated. Markdown output (`plan` plans, `implement` reports) is styled in the terminal with [glamour](https://github.com/charmbracelet/glamour) (the rendering engine behind [glow](https://github.com/charmbracelet/glow)); when piped or redirected, the raw markdown is passed through unchanged.

### Development workflow

`prepare` and `finalize` wrap the git/gh flow around a ticket:

- `xynapse prepare <ref> -b main` — creates a feature branch from the base branch. The branch name is the `git.branch_template` expanded with ticket placeholders (default `feature-v5/{Key}`); pass `--template` to override. It refuses to branch from a dirty working tree unless `--force` is given, and idempotently checks out the branch if it already exists.
- `xynapse finalize <ref> [-b main] [--pr]` — `git add -A` + `git commit -m "<KEY>: <summary>"` (override with `--message`), then `git push -u origin <current branch>`. With `--pr` (requires `--base`) it opens a pull request via `gh` with a `Closes <KEY>` trailer and a link back to the ticket. It refuses to run on the base branch, and marks the ticket's plan `done` on success.

Branch template placeholders:

| Placeholder | Value | Example |
|---|---|---|
| `{Key}` / `{TicketKey}` | full ticket key | `MERADIO-123` |
| `{Project}` | project key | `MERADIO` |
| `{Number}` | numeric part | `123` |
| `{Board}` | board id from `defaults.board_id` | `561` |
| `{Summary}` | slugified summary | `add-stale-detection` |

For example, `{Project}/{TicketKey}` produces `MERADIO/MERADIO-123`. Unknown placeholders and missing required values (`{Key}`, `{Project}`, `{Number}`) are rejected.

### Cache expiration

When cached data is older than `expiration.hours`, `get-ticket` and `get-sprint` automatically re-pull from Jira before printing. Set `expiration.hours` to `0` to disable.

### Storage layout

Fetched tickets are stored under `storage/<PROJECT>/<KEY>.yml`, with sprint ticket lists tracked in `storage/<PROJECT>/sprints/current.yml`. When `board_id` is configured, `pull-sprint` looks up the active sprint via the Jira Agile API and stores its `sprint_id` and `sprint_name` in the manifest. Each ticket keeps both the raw ADF description JSON (`description`) and a flattened plain-text copy (`description_text`). Implementation plans from `plan` are saved under `storage/plans/<KEY>.md`, with their lifecycle `status` stored in the file's YAML frontmatter.

## Help

```sh
./bin/xynapse --help
```
