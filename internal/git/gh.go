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
