package opencode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	// Stream, when non-nil, enables live mode: tool activity (bash commands,
	// file edits, reads, etc.) is written here as opencode executes it, and
	// opencode's own stderr is forwarded to os.Stderr. Assistant text is NOT
	// streamed — the JSON event for a text part only arrives once complete,
	// so callers print the final result (e.g. via RenderMD) instead.
	Stream io.Writer
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
// When opts.Stream is set, tool activity is streamed to it live while the
// full stdout is still captured and returned.
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

	if opts.Stream == nil {
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

	return runStreaming(cmd, opts.Stream)
}

// runStreaming runs the already-configured command, capturing stdout while
// streaming tool activity to w and opencode's stderr to os.Stderr.
func runStreaming(cmd *exec.Cmd, w io.Writer) (string, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("opencode run: stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("opencode run: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("opencode run failed to start: %w", err)
	}

	st := newStreamer(w)
	var outBuf bytes.Buffer
	outErrCh := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(stdoutPipe)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			outBuf.Write(sc.Bytes())
			outBuf.WriteByte('\n')
			st.event(sc.Bytes())
		}
		outErrCh <- sc.Err()
	}()

	var errBuf bytes.Buffer
	errErrCh := make(chan error, 1)
	go func() {
		_, _ = io.Copy(io.MultiWriter(os.Stderr, &errBuf), stderrPipe)
		errErrCh <- nil
	}()

	runErr := cmd.Wait()
	scanErr := <-outErrCh
	<-errErrCh

	if runErr != nil || scanErr != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = strings.TrimSpace(outBuf.String())
		}
		if runErr != nil {
			return "", fmt.Errorf("opencode run failed: %w: %s", runErr, truncate(msg, 500))
		}
		return "", fmt.Errorf("opencode run: reading output: %w", scanErr)
	}
	return outBuf.String(), nil
}

// streamer renders live activity from the opencode JSON event stream.
type streamer struct {
	w    io.Writer
	seen map[string]bool // tool part ids already printed
}

func newStreamer(w io.Writer) *streamer {
	return &streamer{w: w, seen: map[string]bool{}}
}

// event inspects a single NDJSON line. Tool events are printed once per part
// (as "  [tool] title"), so a running→completed transition is not repeated.
func (s *streamer) event(line []byte) {
	if len(line) == 0 || line[0] != '{' {
		return
	}
	var ev struct {
		Type string `json:"type"`
		Part *struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Tool  string `json:"tool"`
			State *struct {
				Status string `json:"status"`
				Title  string `json:"title"`
			} `json:"state"`
		} `json:"part"`
	}
	if err := json.Unmarshal(line, &ev); err != nil {
		return
	}
	if ev.Type != "tool_use" || ev.Part == nil || ev.Part.Type != "tool" || ev.Part.State == nil {
		return
	}
	title := strings.TrimSpace(ev.Part.State.Title)
	if title == "" {
		return
	}
	if s.seen[ev.Part.ID] {
		return
	}
	s.seen[ev.Part.ID] = true

	label := ev.Part.Tool
	if label == "" {
		label = "tool"
	}
	fmt.Fprintf(s.w, "  [%s] %s\n", label, title)
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
