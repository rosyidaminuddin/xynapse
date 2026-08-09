package opencode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Options configures a single `opencode run` invocation.
type Options struct {
	Bin         string // path or name on PATH (e.g. "opencode")
	Dir         string // working directory (target repo); empty = cwd
	Model       string // optional provider/model override
	AutoApprove bool   // pass --auto so the agent can act without prompting
	Prompt      string // the message to run
}

// Locate resolves the opencode binary, erroring with an install hint if
// it cannot be found on PATH.
func Locate(bin string) (string, error) {
	if bin == "" {
		bin = "opencode"
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("opencode binary %q not found on PATH (install it via `brew install opencode` or the install script): %w", bin, err)
	}
	return path, nil
}

// Run executes `opencode run` in non-interactive mode and returns the raw
// stdout. Use ExtractText to turn the JSON event stream into the final text.
func Run(opts Options) (string, error) {
	bin, err := Locate(opts.Bin)
	if err != nil {
		return "", err
	}

	args := []string{"run"}
	if opts.Dir != "" {
		args = append(args, "--dir", opts.Dir)
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.AutoApprove {
		args = append(args, "--auto")
	}
	args = append(args, "--format", "json")
	args = append(args, opts.Prompt)

	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("opencode run failed: %w: %s", err, truncate(msg, 500))
	}
	return stdout.String(), nil
}

// ExtractText parses the `--format json` event stream and concatenates the
// assistant's text parts. Different opencode versions emit text either as
// top-level `text` events or as `message.part.updated` events, both carrying
// part.type == "text". If the output is not JSON events (e.g. an older
// opencode), it falls back to the raw output.
func ExtractText(out string) string {
	var sb strings.Builder
	parsed := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var ev struct {
			Type string `json:"type"`
			Part *struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"part"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		parsed = true
		if ev.Part != nil && ev.Part.Type == "text" && ev.Part.Text != "" {
			sb.WriteString(ev.Part.Text)
		}
	}
	if parsed && sb.Len() > 0 {
		return strings.TrimSpace(sb.String())
	}
	return strings.TrimSpace(out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
