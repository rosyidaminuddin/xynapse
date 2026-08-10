package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Git runs git commands inside a repository directory.
type Git struct {
	dir string
	bin string
}

// New returns a Git runner rooted at dir. An empty bin defaults to "git".
func New(dir, bin string) *Git {
	if bin == "" {
		bin = "git"
	}
	return &Git{dir: dir, bin: bin}
}

// run executes a git command and returns its trimmed stdout.
func (g *Git) run(args ...string) (string, error) {
	cmd := exec.Command(g.bin, args...)
	cmd.Dir = g.dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, truncate(msg, 500))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// CurrentBranch returns the checked-out branch name.
func (g *Git) CurrentBranch() (string, error) {
	return g.run("rev-parse", "--abbrev-ref", "HEAD")
}

// IsDirty reports whether the working tree has uncommitted changes.
func (g *Git) IsDirty() (bool, error) {
	out, err := g.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// StatusPorcelain returns the raw `git status --porcelain` output.
func (g *Git) StatusPorcelain() (string, error) {
	return g.run("status", "--porcelain")
}

// BranchExists reports whether a local branch exists.
func (g *Git) BranchExists(name string) bool {
	_, err := g.run("rev-parse", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

// IsInsideWorkTree reports whether dir is inside a git working tree.
func (g *Git) IsInsideWorkTree() (bool, error) {
	out, err := g.run("rev-parse", "--is-inside-work-tree")
	if err != nil {
		return false, err
	}
	return out == "true", nil
}

// LocalBranches returns the short names of all local branches.
func (g *Git) LocalBranches() ([]string, error) {
	out, err := g.run("for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// CommitSubjects returns the subject line of every commit reachable from any
// local ref. An empty repository yields an empty list.
func (g *Git) CommitSubjects() ([]string, error) {
	out, err := g.run("log", "--all", "--format=%s")
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits") {
			return nil, nil
		}
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// Checkout switches to an existing branch or commit.
func (g *Git) Checkout(ref string) error {
	_, err := g.run("checkout", ref)
	return err
}

// CreateBranch creates and checks out a new branch from HEAD.
func (g *Git) CreateBranch(name string) error {
	_, err := g.run("checkout", "-b", name)
	return err
}

// AddAll stages all changes (including untracked files).
func (g *Git) AddAll() error {
	_, err := g.run("add", "-A")
	return err
}

// Commit creates a commit with the given message.
func (g *Git) Commit(message string) error {
	_, err := g.run("commit", "-m", message)
	return err
}

// Push pushes the branch to origin, setting the upstream.
func (g *Git) Push(branch string) error {
	_, err := g.run("push", "-u", "origin", branch)
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
