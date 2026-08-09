package git

import (
	"bytes"
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
