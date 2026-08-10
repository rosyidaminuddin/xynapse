package command

import (
	"fmt"
	"strings"
	"time"

	"xynapse/internal/config"
	"xynapse/internal/git"
	"xynapse/internal/models"
	"xynapse/internal/storage"
)

// statusUnknown marks a git/PR check that could not run (not in a repo, git
// or gh missing, etc.). get-sprint never fails because of these.
const statusUnknown = "?"

// gitSnapshot is a best-effort snapshot of the project repository's state for
// the sprint tickets. Each check is either available or marked failed so the
// caller can distinguish "no match" from "couldn't tell".
type gitSnapshot struct {
	available  bool
	branchesOK bool
	branches   []string
	subjectsOK bool
	subjects   []string
	prOK       bool
	prStates   map[string]git.PRInfo
}

func GetSprint(cfg *config.Config, types []string, unplanned bool) error {
	s := storage.NewStorage(cfg.Storage.Base)

	logStep(cfg.Verbose, "reading sprint manifest for project %s from %s", cfg.Defaults.Project, cfg.Storage.Base)
	manifest, err := s.ReadSprintManifest(cfg.Defaults.Project)
	if err != nil {
		return err
	}
	logStep(cfg.Verbose, "sprint manifest loaded (fetched_at=%s, %d tickets)", manifest.FetchedAt.Format(time.RFC3339), len(manifest.TicketKeys))

	if expired := cfg.Expiration.Duration(); expired > 0 && time.Since(manifest.FetchedAt) > expired {
		logStep(cfg.Verbose, "cached sprint is stale (older than %s), refreshing from Jira", expired)
		if err := PullSprint(cfg, types); err != nil {
			return err
		}
	}

	tickets, err := s.ReadSprintTickets(cfg.Defaults.Project)
	if err != nil {
		return err
	}

	if len(types) > 0 {
		tickets = filterByType(tickets, types)
		logStep(cfg.Verbose, "filtered to %d ticket(s) of type(s) %v", len(tickets), types)
	}

	snap := collectGitSnapshot(cfg)

	views := make([]SprintTicket, 0, len(tickets))
	for _, ticket := range tickets {
		derived, local := sprintBranch(cfg, ticket, snap)
		views = append(views, SprintTicket{
			Ticket:    ticket,
			Plan:      planStatus(s, cfg.Defaults.Project, ticket.Key, ticket.UpdatedAt),
			Finalized: finalizedStatus(snap, ticket.Key),
			PR:        prStatus(snap, effectiveBranch(local, derived)),
			Target:    prTarget(snap, effectiveBranch(local, derived)),
			Branch:    local,
		})
	}

	if unplanned {
		views = filterUnplanned(views)
		logStep(cfg.Verbose, "filtered to %d ticket(s) without a plan", len(views))
	}

	if err := printSprintTickets(cfg.Defaults.OutputFormat, views); err != nil {
		return fmt.Errorf("failed to print sprint tickets: %w", err)
	}
	return nil
}

// collectGitSnapshot runs the git/gh checks in the project's repository
// directory (git.dir, else the current working directory). Individual
// failures are logged in verbose mode and leave that check marked failed;
// the command never errors here.
func collectGitSnapshot(cfg *config.Config) gitSnapshot {
	snap := gitSnapshot{}

	repoDir := cfg.Git.Dir
	if repoDir == "" {
		logStep(cfg.Verbose, "no git.dir configured, inspecting current working directory")
	} else {
		logStep(cfg.Verbose, "inspecting git repository at %s", repoDir)
	}

	g := git.New(repoDir, "")
	inside, err := g.IsInsideWorkTree()
	if err != nil || !inside {
		logStep(cfg.Verbose, "not inside a git working tree; git/PR status unavailable")
		return snap
	}
	snap.available = true

	if branches, err := g.LocalBranches(); err != nil {
		logStep(cfg.Verbose, "could not list local branches: %v", err)
	} else {
		snap.branches, snap.branchesOK = branches, true
	}

	if subjects, err := g.CommitSubjects(); err != nil {
		logStep(cfg.Verbose, "could not read commit subjects: %v", err)
	} else {
		snap.subjects, snap.subjectsOK = subjects, true
	}

	if states, err := git.PRStates("gh", repoDir); err != nil {
		logStep(cfg.Verbose, "could not read pull request states: %v", err)
	} else {
		snap.prStates, snap.prOK = states, true
	}
	return snap
}

// sprintBranch returns the template-derived branch for a ticket (used for PR
// lookup even when the branch is not checked out locally) and the locally
// detected branch shown in the table: the derived name when it exists, else a
// local branch whose name contains the ticket key, else "-".
func sprintBranch(cfg *config.Config, ticket *models.Ticket, snap gitSnapshot) (derived, local string) {
	if !snap.available {
		return "", statusUnknown
	}

	tmpl := git.ResolveTemplate(cfg.Git.BranchTemplate, cfg.Git.BranchTemplates, ticket.Type)
	if tmpl == "" {
		tmpl = "feature-v5/{Key}"
	}
	derived, err := git.ExpandTemplate(tmpl, git.TemplateVars{
		Key:     ticket.Key,
		Project: ticket.Project,
		Number:  ticketNumber(ticket.Key),
		Board:   cfg.Defaults.BoardID,
		Summary: ticket.Summary,
	})
	if err != nil {
		logStep(cfg.Verbose, "could not expand branch template for %s: %v", ticket.Key, err)
		return "", statusUnknown
	}

	if !snap.branchesOK {
		return derived, statusUnknown
	}
	if containsString(snap.branches, derived) {
		return derived, derived
	}
	want := strings.ToLower(ticket.Key)
	for _, b := range snap.branches {
		if strings.Contains(strings.ToLower(b), want) {
			return derived, b
		}
	}
	return derived, "-"
}

// finalizedStatus reports whether a commit mentioning the ticket key exists
// in the repository: "yes", "no", or "?" when the check could not run.
func finalizedStatus(snap gitSnapshot, key string) string {
	if !snap.available || !snap.subjectsOK {
		return statusUnknown
	}
	want := strings.ToLower(key) + ":"
	for _, s := range snap.subjects {
		if strings.Contains(strings.ToLower(s), want) {
			return "yes"
		}
	}
	return "no"
}

// prStatus returns the pull request state for the derived branch: "merged",
// "open", "closed", "none" (branch known but no PR), or "?" when the check
// could not run.
func prStatus(snap gitSnapshot, derived string) string {
	if !snap.available || !snap.prOK || derived == "" {
		return statusUnknown
	}
	if pr, ok := snap.prStates[derived]; ok {
		return pr.State
	}
	return "none"
}

// prTarget returns the pull request's target branch for the derived branch:
// the base branch (e.g. "main"), "-" when there is no PR, or "?" when the
// check could not run.
func prTarget(snap gitSnapshot, derived string) string {
	if !snap.available || !snap.prOK || derived == "" {
		return statusUnknown
	}
	if pr, ok := snap.prStates[derived]; ok {
		if pr.Base == "" {
			return "-"
		}
		return pr.Base
	}
	return "-"
}

// effectiveBranch picks the branch to look up in the PR states: the locally
// detected branch when there is one (it may differ from the template-derived
// name after a --template override), otherwise the derived name.
func effectiveBranch(local, derived string) string {
	if local != "-" && local != statusUnknown {
		return local
	}
	return derived
}

// ticketNumber returns the numeric part of a ticket key ("PROJ-1" -> "1").
func ticketNumber(key string) string {
	if i := strings.LastIndex(key, "-"); i >= 0 {
		return key[i+1:]
	}
	return key
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// filterUnplanned keeps only tickets that have no saved plan.
func filterUnplanned(views []SprintTicket) []SprintTicket {
	filtered := make([]SprintTicket, 0, len(views))
	for _, v := range views {
		if v.Plan == PlanNone {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// filterByType keeps only tickets whose type matches one of the given types
// (case-insensitive). An empty type list returns all tickets unchanged.
func filterByType(tickets []*models.Ticket, types []string) []*models.Ticket {
	if len(types) == 0 {
		return tickets
	}

	allowed := make(map[string]bool, len(types))
	for _, t := range types {
		allowed[strings.ToLower(strings.TrimSpace(t))] = true
	}

	filtered := make([]*models.Ticket, 0, len(tickets))
	for _, ticket := range tickets {
		if allowed[strings.ToLower(ticket.Type)] {
			filtered = append(filtered, ticket)
		}
	}
	return filtered
}
