package git

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// PROptions configures a `gh pr create` invocation.
type PROptions struct {
	Base  string // target branch
	Head  string // source branch
	Title string
	Body  string
}

// PRInfo describes a pull request's state for get-sprint output.
type PRInfo struct {
	// State is the PR state: "merged", "open", or "closed".
	State string
	// Base is the target branch the PR merges into (e.g. "main").
	Base string
}

// PRDetails is the full review/merge state of a single pull request, used to
// gate the drive workflow's ticket step.
type PRDetails struct {
	// Number is the pull request number.
	Number int
	// State is the PR state: "merged", "open", or "closed". "none" is used
	// when no PR exists for the head branch.
	State string
	// ReviewDecision is the review state: "APPROVED", "CHANGES_REQUESTED",
	// "REVIEW_REQUIRED", or "" when there are no reviews.
	ReviewDecision string
	// Mergeable reports whether the PR is currently mergeable.
	Mergeable string
	// URL is the pull request page URL.
	URL string
}

// PRStates returns the state and target branch of every pull request in the
// repository, keyed by its head branch. A single `gh pr list` call covers all
// branches.
func PRStates(bin string, dir string) (map[string]PRInfo, error) {
	if bin == "" {
		bin = "gh"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("gh binary %q not found on PATH (install it via `brew install gh`): %w", bin, err)
	}

	cmd := exec.Command(path, "pr", "list", "--state", "all", "--json", "number,headRefName,state,baseRefName")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return nil, fmt.Errorf("gh pr list failed: %w: %s", err, truncate(msg, 500))
	}

	var prs []struct {
		HeadRefName string `json:"headRefName"`
		State       string `json:"state"`
		BaseRefName string `json:"baseRefName"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &prs); err != nil {
		return nil, fmt.Errorf("failed to parse gh pr list output: %w", err)
	}

	states := make(map[string]PRInfo, len(prs))
	for _, pr := range prs {
		states[pr.HeadRefName] = PRInfo{State: strings.ToLower(pr.State), Base: pr.BaseRefName}
	}
	return states, nil
}

// CreatePR creates a pull request via the gh CLI and returns the PR URL.
// The gh binary must be installed and authenticated.
func CreatePR(bin string, dir string, opts PROptions) (string, error) {
	if bin == "" {
		bin = "gh"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("gh binary %q not found on PATH (install it via `brew install gh`): %w", bin, err)
	}

	args := []string{"pr", "create", "--base", opts.Base, "--title", opts.Title, "--body", opts.Body}
	if opts.Head != "" {
		args = append(args, "--head", opts.Head)
	}

	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("gh pr create failed: %w: %s", err, truncate(msg, 500))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// PRView returns the review and merge state of the pull request whose head
// branch is head. When no PR exists for the branch, it returns a PRDetails
// with State "none" and a nil error so callers can treat "not found" as a
// normal state rather than a failure.
func PRView(bin string, dir string, head string) (*PRDetails, error) {
	if bin == "" {
		bin = "gh"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return nil, fmt.Errorf("gh binary %q not found on PATH (install it via `brew install gh`): %w", bin, err)
	}

	cmd := exec.Command(path, "pr", "view", head, "--json", "number,state,reviewDecision,mergeable,url")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if strings.Contains(strings.ToLower(msg), "no pull requests") || strings.Contains(strings.ToLower(msg), "could not resolve") {
			return &PRDetails{State: "none"}, nil
		}
		return nil, fmt.Errorf("gh pr view failed: %w: %s", err, truncate(msg, 500))
	}

	var pr struct {
		Number         int    `json:"number"`
		State          string `json:"state"`
		ReviewDecision string `json:"reviewDecision"`
		Mergeable      string `json:"mergeable"`
		URL            string `json:"url"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &pr); err != nil {
		return nil, fmt.Errorf("failed to parse gh pr view output: %w", err)
	}

	return &PRDetails{
		Number:         pr.Number,
		State:          strings.ToLower(pr.State),
		ReviewDecision: strings.ToUpper(pr.ReviewDecision),
		Mergeable:      pr.Mergeable,
		URL:            pr.URL,
	}, nil
}
