# xynapse

A CLI to manage Jira tickets locally and drive them through a full development workflow.

- **Pull & cache** — fetch tickets and sprints from the Jira REST API and store them as YAML, so `get` commands read from disk without hitting the server.
- **Query** — view tickets (`get-ticket`) and sprint boards (`get-sprint`) with plan/PR/branch status in plain text, JSON, or YAML.
- **Plan & implement** — delegate to opencode to generate a step-by-step implementation plan and execute it in a repo, with plan confirmations answered in the terminal.
- **Ship** — `prepare` creates a feature branch, `finalize` commits, pushes, and opens a pull request via `gh`.
- **Update Jira** — `transition` moves tickets between statuses, `assignee` manages owners, and `drive` ties it all together.
- **Drive** — one command runs a ticket through the whole pipeline (branch → plan → implement → test → lint → finalize → ticket), waits for the PR to merge, then updates Jira automatically.

## Requirements

- Go 1.26+
- A Jira Cloud account with API token access
- [opencode](https://opencode.ai) CLI for the `plan` and `implement` commands
- [gh](https://cli.github.com) CLI (installed and authenticated) for `finalize --pr` and the `drive` command. Optional for `get-sprint`: without it the PR column shows `?` and the command still works.

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
  api_token: ""          # or set via JIRA_API_TOKEN
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

git:
  dir: ""                            # target repo directory for plan/implement/prepare/finalize/get-sprint (default: cwd)
  branch_template: "feature-v5/{Key}"  # branch naming for `prepare`; see placeholders below
  branch_templates:                     # optional per-type overrides (matched case-insensitively)
    Bug: "fix-v5/{Key}"
    Epic: "epic/{Key}"

# drive workflow: optional, only needed by `xynapse drive`
workflow:
  test_command: "go test ./..."      # command the test step runs in the repo
  lint_command: "go vet ./..."       # optional lint step
  test_status: "In Review"           # Jira status the ticket moves to after its PR merges
  base_branch: "main"                # branch feature branches are created from
  target_branch: "develop"           # PR merge target (defaults to base_branch)
  comment_template: "PR: {url}"      # comment posted on the ticket after merge; {key}/{summary}/{url}/{branch}
  autopilot: false                   # behave as if --yes was always passed

projects:
  MERADIO:
    board_id: "561"
    git:
      dir: "~/work/meradio"          # optional per-project repo directory
      branch_template: "feature-v5/{Key}"
      branch_templates:
        Bug: "fix-v5/{Key}"
        Epic: "epic-v5/{Key}"
    # Per-project workflow is optional and merges over the top-level workflow
    # field-by-field: only the fields set here override the defaults above.
    # Here MERADIO drives its PRs into develop and moves tickets to "QA" after
    # merge; everything else (test/lint commands, base_branch, autopilot) is
    # inherited from the top-level workflow.
    workflow:
      target_branch: "develop"       # PRs target develop instead of main
      test_status: "QA"              # tickets move to QA after their PR merges
  ALPHA:
    board_id: "99"
    git:
      branch_template: "release/{Key}"
    # No workflow section: ALPHA uses the top-level workflow as-is.
```

Credentials are resolved from (highest priority first):

1. Environment variables `JIRA_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`
2. Values in `~/.config/xynapse/config.yaml`

> Never commit credentials. `~/.config/xynapse/` is per-user and not shared.

Set `XYNAPSE_CONFIG_DIR` to override the config directory and `XYNAPSE_STORAGE` to override the storage directory.

### Inspecting and editing config

The `config` subcommand reads and writes the config file without needing valid Jira credentials:

```sh
./bin/xynapse config                  # print the full effective config (api_token redacted)
./bin/xynapse config path             # print the config file path
./bin/xynapse config get jira.url     # print the effective value of a key
./bin/xynapse config set defaults.project ALPHA
./bin/xynapse config set expiration.hours 48
./bin/xynapse config set opencode.auto_approve true
./bin/xynapse config set projects.MERADIO.board_id 561
```

- Keys are dot-separated paths into the config, including map entries (`git.branch_templates.Bug`, `projects.MERADIO.git.branch_template`).
- `config get` returns the **effective** value — environment overlays (`JIRA_*`) and defaults are applied, so an unset `git.branch_template` still prints `feature-v5/{Key}`.
- `config set` creates the file and any nested keys automatically, keeps existing YAML comments, and stores values as typed scalars (`true` → boolean, `48` → number, otherwise string). Setting a key to its current value is a no-op.
- Bare `config` prints the full effective config with `jira.api_token` redacted as `REDACTED`. `config get jira.api_token` still prints the real token.

### Multiple projects

Define a `projects:` map to give each project its own board and branching strategy:

- When **one** project is configured it is selected automatically.
- With **multiple** projects, set `defaults.project` (or pass `-p/--project <KEY>`) to pick one.
- Project keys are matched case-insensitively (`-p meradio` selects `MERADIO`).
- `-p` accepts any key, configured or not. Unconfigured keys fall back to the global settings.
- Per-project `board_id` overrides the top-level one. A per-project `git` section is **merged** over the top-level `git` section field-by-field: only the fields a project sets are overridden, empty fields inherit the top-level value (e.g. `git.dir`), and an entirely empty `branch_template` falls back to the default `feature-v5/{Key}`.
- A per-project `git.branch_templates` **replaces** the global map for that project.
- A per-project `workflow` section is **merged** over the top-level `workflow` the same way `git` is: only the fields a project sets are overridden, empty fields inherit the top-level value, and an unset `target_branch` falls back to `base_branch`. Projects without a `workflow` section use the top-level workflow unchanged.

Storage is already per-project (`storage/<PROJECT>/...`), so tickets from different projects never collide.

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

`get-sprint` shows one row per cached ticket with these columns:

- `KEY` — ticket key
- `STATUS` — Jira status
- `FINALIZED` — `yes` when a local commit subject contains `<KEY>:` (see `finalize`); `no` otherwise
- `PR` — pull-request state from `gh` for the ticket's branch (`merged`/`open`/`closed`/`none`)
- `TARGET` — the PR's target (base) branch, e.g. `main` (`-` when there is no PR)
- `BRANCH` — the local branch for the ticket, derived from the branch template or scanned by key
- `PLAN` — plan state (`yes`/`stale`/`no`)
- `TYPE`, `ASSIGNEE`, `SUMMARY` — ticket details

The `FINALIZED`, `PR`, and `BRANCH` columns come from the configured git repo (`git.dir`,
defaulting to cwd); outside a repo they show `?`.

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

# List the statuses a ticket can transition to
./bin/xynapse transition MERADIO-123

# Move a Jira ticket to a new status (by target status or transition name)
./bin/xynapse transition MERADIO-123 "In Progress"

# Force a specific transition when several lead to the same status
./bin/xynapse transition MERADIO-123 --id 41

# Show the current assignee (fetched live from Jira)
./bin/xynapse assignee MERADIO-123

# Assign a ticket by display name or email
./bin/xynapse assignee MERADIO-123 "Jane Doe"
./bin/xynapse assignee MERADIO-123 jane@corp.com

# Clear the assignee
./bin/xynapse assignee MERADIO-123 unassigned

# Create a feature branch (feature-v5/MERADIO-123) from main
./bin/xynapse prepare MERADIO-123 -b main

# Commit, push, and open a pull request against main
./bin/xynapse finalize MERADIO-123 -b main --pr

# Drive a ticket through the whole workflow (see "Driving a ticket" below)
./bin/xynapse drive MERADIO-123 --yes
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
- `-p`, `--project <KEY>` — project key to use (defaults to `defaults.project` or the single configured project; see [Multiple projects](#multiple-projects))
- `-t`, `--type <types>` — comma-separated issue types (e.g. `Story,Bug,Epic`). For `pull-sprint` it is applied as a JQL `issuetype in (...)` filter on the server; for `get-sprint` it filters the locally cached tickets.
- `-o`, `--output <format>` — output format for `get-ticket`: `table`, `json`, or `yaml` (overrides config)
- `-f`, `--force` — skip the confirmation prompt (`clear-cache`, `implement`) or the dirty-tree check (`prepare`)
- `--dir <repo>` — target repo directory for `plan`/`implement`/`prepare`/`finalize` (defaults to `git.dir`, then cwd)
- `--model <provider/model>` — override the opencode model for `plan`/`implement`
- `-b`, `--branch <branch>` — check out this branch in the target repo before `plan` analyzes the ticket
- `--plan <path>` — use a specific plan file for `implement` (default: `<storage>/plans/<KEY>.md`)
- `-b`, `--base <branch>` — base branch: required for `prepare` (branch from); for `finalize` it is the PR target and makes finalize refuse to commit on it
- `--template <tmpl>` — override the branch template for `prepare`
- `--message <msg>` — commit message for `finalize` (default: `"<KEY>: <summary>"`)
- `--pr` — create a pull request with `gh` after `finalize` pushes (requires `--base`)
- `--set <status>` — update a plan's status with `status` (not started, in progress, in review, done)
- `--id <id>` — force a specific transition for `transition` (disambiguates multiple transitions that lead to the same status)
- `-y`, `--yes` — autopilot for `drive`: confirmations use suggested defaults, prompts are skipped
- `--base <branch>` — PR target branch for `drive` (defaults to `workflow.target_branch`, then `base_branch`)
- `--status <status>` — Jira status for `drive`'s ticket step (defaults to `workflow.test_status`)
- `--dry-run` — print the steps `drive` would run without executing them
- `--step <name>` — run a single `drive` step (branch, plan, implement, test, lint, finalize, ticket)
- `--from <name>` / `--to <name>` — run an inclusive range of `drive` steps

### Implementation plans

`plan` and `implement` delegate to the [opencode](https://opencode.ai) CLI and two skills shipped in this repo (`.opencode/skills/`, synced to `~/.config/opencode/skills/` by `install.sh`):

- `xynapse plan <ref>` — fetches/refreshes the ticket, runs the `analyze-ticket` skill to produce a step-by-step implementation plan, and saves it to `<storage>/plans/<KEY>.md`.
- `xynapse implement <ref>` — runs the `implement-plan` skill in the target repo to execute the saved plan. The agent leaves changes in the working tree; it does not commit or push.
- `xynapse show-plan <ref>` — displays a ticket's saved plan from `<storage>/plans/<KEY>.md` as styled markdown (works without a cached ticket).
- `xynapse status <ref> [--set <status>]` — show or update a plan's status.

Each plan file carries a lifecycle **status** in its YAML frontmatter: `not started` (set by `plan`), `in progress` (set by `implement`), `in review`, and `done` (set by `finalize`). Update it manually with `xynapse status <ref> --set in_review`. `show-plan` displays the status as a header.

A plan is **stale** when the ticket changed on Jira after it was written. `get-sprint` reflects this in the PLAN column (`yes`/`stale`/`no`), and `show-plan` warns on stderr. `implement` warns and asks for confirmation before running a stale plan; pass `--force` to skip the prompt. Staleness is only checked when the ticket is cached — plan-only workflows keep working offline.

### Confirmations and decisions

The `analyze-ticket` skill never pauses for input during implementation. Instead, any decision the implementer would otherwise have to ask about is written into the plan as a numbered question in the `## Confirmations` section, each with a suggested default:

```markdown
## Confirmations

1. Which database should the new table live in? (default: PostgreSQL)
2. Should the migration run automatically on deploy? (default: no)
```

`xynapse implement <ref>` checks the plan before launching opencode. Unanswered confirmations are prompted for in the terminal:

```sh
plan requires 2 confirmation(s) before implementing. Answer each question; press Enter to accept the suggested default.

  1. Which database should the new table live in? [PostgreSQL]: postgres
  2. Should the migration run automatically on deploy? [no]: yes
```

- Press **Enter** to accept the suggested default; type a value to override it. `Ctrl-C` cancels and **nothing is implemented**.
- Answers are recorded in a `## Decisions` section in the plan file (e.g. `- 1. postgres`), so `show-plan` displays them and re-running `implement` does not ask again.
- The `implement-plan` skill treats those decisions as hard constraints and refuses to implement a plan that still has unanswered confirmations. `--force` does **not** skip confirmation prompts.
- If the plan has no `## Confirmations` section (or it says `None.`), `implement` runs as before without prompting.

While opencode works, its activity is streamed live to stderr — each tool call (bash command, file edit, read, etc.) is printed as `  [tool] title` the moment it runs — so you can watch what the agent is doing in the terminal. Assistant text is shown once, when the command finishes.

Review the plan before executing. Skills require the opencode CLI to be installed and authenticated. Markdown output (`plan` plans, `implement` reports) is styled in the terminal with [glamour](https://github.com/charmbracelet/glamour) (the rendering engine behind [glow](https://github.com/charmbracelet/glow)); when piped or redirected, the raw markdown is passed through unchanged.

### Driving a ticket

`xynapse drive <ref>` runs a single ticket through the whole pipeline, recording each completed step per ticket so interrupted runs resume where they stopped:

```
branch -> plan -> implement -> test -> lint -> finalize -> ticket
```

- **branch** — creates/checks out the feature branch (from `workflow.base_branch`, or `--base`).
- **plan** — generates the plan if none exists or it is stale. A stale plan prompts for confirmation; `--force` or `--yes` regenerates without asking.
- **implement** — runs the saved plan via opencode in the repo. Its confirmation prompts are answered interactively, or with suggested defaults under `--yes`.
- **test / lint** — runs `workflow.test_command` / `workflow.lint_command` in the repo. A failure records the step and blocks **finalize** until fixed; `--force` bypasses the gate. Empty `test_command` skips the step.
- **finalize** — commits, pushes, and opens a PR against `workflow.target_branch` (falls back to `base_branch`). An explicit `--base` overrides both.
- **ticket** — waits until the PR is **merged**, then assigns the ticket, transitions it to `workflow.test_status` (or `--status`), and posts the `workflow.comment_template` comment (default `PR: {url}\n\nCloses {key}`). Until the PR merges, drive stops here and tells you to re-run once it is merged.

```sh
# Autopilot: confirmations use suggested defaults, no prompts
./bin/xynapse drive MERADIO-123 --yes

# Interactive: answer each confirmation/assignment/comment prompt as it comes
./bin/xynapse drive MERADIO-123

# Drive from a specific repo or model, or PR against a specific base
./bin/xynapse drive MERADIO-123 --dir ~/work/myproject --base main --model anthropic/claude-sonnet-4

# Run a single step or a range of steps
./bin/xynapse drive MERADIO-123 --step implement
./bin/xynapse drive MERADIO-123 --from test --to finalize

# Preview the steps without executing anything
./bin/xynapse drive MERADIO-123 --dry-run
```

Notes:

- Steps are recorded under `<storage>/drive/<PROJECT>/<KEY>.yml`. `clear-cache` removes them with the rest of the cache.
- The **ticket** step only runs after the PR merges; a re-run checks the current PR state via `gh` before touching Jira. Until then the drive state stays open for that step.
- `--yes` is the equivalent of setting `workflow.autopilot: true` in config. The **assignee** prompt and **comment** prompt are skipped in autopilot mode (current assignee is kept, template comment is posted).
- Drive refuses to run outside a git working tree. Configure `git.dir` (or pass `--dir`) so it targets the right repo.

### Development workflow

`prepare` and `finalize` wrap the git/gh flow around a ticket:

- `xynapse prepare <ref> -b main` — creates a feature branch from the base branch. The branch name is the `git.branch_template` expanded with ticket placeholders (default `feature-v5/{Key}`); a `git.branch_templates.<TYPE>` entry overrides it for that issue type (e.g. `Bug: "fix-v5/{Key}"`), and `--template` overrides everything. It refuses to branch from a dirty working tree unless `--force` is given, and idempotently checks out the branch if it already exists.
- `xynapse finalize <ref> [-b main] [--pr]` — `git add -A` + `git commit -m "<KEY>: <summary>"` (override with `--message`), then `git push -u origin <current branch>`. With `--pr` (requires `--base`) it opens a pull request via `gh` with a `Closes <KEY>` trailer and a link back to the ticket. It refuses to run on the base branch, and marks the ticket's plan `done` on success.

Branch template placeholders:

| Placeholder | Value | Example |
|---|---|---|
| `{Key}` / `{TicketKey}` | full ticket key | `MERADIO-123` |
| `{Project}` | project key | `MERADIO` |
| `{Number}` | numeric part | `123` |
| `{Board}` | board id (`defaults.board_id`, overridable per project) | `561` |
| `{Summary}` | slugified summary | `add-stale-detection` |

For example, `{Project}/{TicketKey}` produces `MERADIO/MERADIO-123`. Unknown placeholders and missing required values (`{Key}`, `{Project}`, `{Number}`) are rejected.

### Cache expiration

When cached data is older than `expiration.hours`, `get-ticket` and `get-sprint` automatically re-pull from Jira before printing. Set `expiration.hours` to `0` to disable.

### Storage layout

Fetched tickets are stored under `storage/<PROJECT>/<KEY>.yml`, with sprint ticket lists tracked in `storage/<PROJECT>/sprints/current.yml`. When `board_id` is configured, `pull-sprint` looks up the active sprint via the Jira Agile API and stores its `sprint_id` and `sprint_name` in the manifest. Each ticket keeps both the raw ADF description JSON (`description`) and a flattened plain-text copy (`description_text`). Implementation plans from `plan` are saved under `storage/plans/<KEY>.md`, with their lifecycle `status` stored in the file's YAML frontmatter.

### Jira status transitions

`xynapse transition <ref> [status]` moves a ticket through its Jira workflow via the transitions API:

- With a status argument, the ticket is moved to the matching status. The status is matched case-insensitively against both the transition's target status and its name. If several transitions lead to the same status, an error lists their IDs — pass `--id <id>` to pick one.
- Without an argument, the available transitions (id, transition name, target status) are printed.
- After a successful transition the locally cached ticket is re-fetched, so `get-ticket`, `get-sprint`, and plan-staleness reflect the new status.

### Jira assignee

`xynapse assignee <ref> [user]` shows or updates a ticket's assignee:

- Without a user argument the current assignee is fetched live from Jira.
- A user is resolved via the Jira user search by display name or email address; a case-insensitive exact match on either wins, and multiple fuzzy matches produce an error listing the candidates to refine by.
- A bare account ID (the long alphanumeric Jira user ID) is used directly without a search.
- `unassigned`, `none`, or `-` clears the assignee.
- After a successful update the locally cached ticket is re-fetched.

## Help

```sh
./bin/xynapse --help
```
