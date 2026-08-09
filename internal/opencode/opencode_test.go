package opencode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFakeBin creates an executable shell script on PATH that records its
// args and emits a canned response.
func writeFakeBin(t *testing.T, dir string, script string) string {
	t.Helper()
	path := filepath.Join(dir, "opencode")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake bin: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return path
}

const fakeScript = `#!/bin/sh
echo "$@" > "$FAKE_ARGS_FILE"
cat "$FAKE_OUTPUT_FILE"
`

func TestLocateFound(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, `#!/bin/sh
exit 0
`)
	path, err := Locate("opencode")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if path == "" {
		t.Fatal("Locate returned empty path")
	}
}

func TestLocateMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Locate("definitely-not-a-real-bin-xyz")
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention not found, got %v", err)
	}
}

func TestRunArgsAndOutput(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")
	outputFile := filepath.Join(dir, "output.json")
	writeFakeBin(t, dir, `#!/bin/sh
echo "$@" > "$FAKE_ARGS_FILE"
cat "$FAKE_OUTPUT_FILE"
`)
	t.Setenv("FAKE_ARGS_FILE", argsFile)
	t.Setenv("FAKE_OUTPUT_FILE", outputFile)

	if err := os.WriteFile(outputFile, []byte(`{"type":"message.part.updated","part":{"type":"text","text":"hello world"}}
{"type":"session.idle","part":null}
`), 0o644); err != nil {
		t.Fatalf("write output: %v", err)
	}

	out, err := Run(Options{
		Bin:         "opencode",
		Dir:         "/work/repo",
		Model:       "m1",
		AutoApprove: true,
		Prompt:      "Use the analyze-ticket skill.",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	got := string(args)
	for _, want := range []string{"run", "--dir", "/work/repo", "--model", "m1", "--auto", "--format", "json", "Use the analyze-ticket skill."} {
		if !strings.Contains(got, want) {
			t.Errorf("args missing %q: %s", want, got)
		}
	}

	text := ExtractText(out)
	if text != "hello world" {
		t.Errorf("ExtractText = %q, want %q", text, "hello world")
	}
}

func TestExtractTextFallbackRaw(t *testing.T) {
	out := "plain text not json"
	if got := ExtractText(out); got != out {
		t.Errorf("ExtractText raw fallback = %q, want %q", got, out)
	}
}

func TestExtractTextTextEvents(t *testing.T) {
	out := `{"type":"tool_use","part":{"type":"tool","tool":"bash","state":{"status":"completed"}}}
{"type":"text","part":{"type":"text","text":"Step 1: inspect code"}}
{"type":"text","part":{"type":"text","text":"\nStep 2: implement"}}
{"type":"step_finish","part":{"type":"step-finish"}}
`
	want := "Step 1: inspect code\nStep 2: implement"
	if got := ExtractText(out); got != want {
		t.Errorf("ExtractText = %q, want %q", got, want)
	}
}

func TestExtractTextMessagePartUpdated(t *testing.T) {
	out := `{"type":"message.part.updated","part":{"type":"text","text":"hello world"}}
{"type":"session.idle","part":null}
`
	if got := ExtractText(out); got != "hello world" {
		t.Errorf("ExtractText = %q, want %q", got, "hello world")
	}
}

func TestRunFailureReportsStderr(t *testing.T) {
	dir := t.TempDir()
	writeFakeBin(t, dir, `#!/bin/sh
echo "boom" >&2
exit 3
`)
	_, err := Run(Options{Bin: "opencode", Prompt: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should include stderr, got %v", err)
	}
}
