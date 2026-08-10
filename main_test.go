package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	f()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestConfigCommandBypassesValidation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XYNAPSE_CONFIG_DIR", tmp)

	root := newRootCmd()
	root.SetArgs([]string{"config", "path"})
	if err := root.Execute(); err != nil {
		t.Fatalf("config path should work without valid config: %v", err)
	}
}

func TestConfigGetBypassesValidation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XYNAPSE_CONFIG_DIR", tmp)
	if err := os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte("defaults:\n  project: ALPHA\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		root := newRootCmd()
		root.SetArgs([]string{"config", "get", "defaults.project"})
		if err := root.Execute(); err != nil {
			t.Fatalf("config get should work without valid config: %v", err)
		}
	})
	if strings.TrimSpace(out) != "ALPHA" {
		t.Errorf("config get = %q, want ALPHA", out)
	}
}

func TestConfigSetWritesFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XYNAPSE_CONFIG_DIR", tmp)

	out := captureStdout(t, func() {
		root := newRootCmd()
		root.SetArgs([]string{"config", "set", "defaults.project", "ALPHA"})
		if err := root.Execute(); err != nil {
			t.Fatalf("config set: %v", err)
		}
	})
	if !strings.Contains(out, "set defaults.project to ALPHA") {
		t.Errorf("set output = %q", out)
	}

	got := captureStdout(t, func() {
		root := newRootCmd()
		root.SetArgs([]string{"config", "get", "defaults.project"})
		if err := root.Execute(); err != nil {
			t.Fatalf("config get: %v", err)
		}
	})
	if strings.TrimSpace(got) != "ALPHA" {
		t.Errorf("config get = %q, want ALPHA", got)
	}
}

func TestConfigDumpRedactsToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XYNAPSE_CONFIG_DIR", tmp)
	if err := os.WriteFile(filepath.Join(tmp, "config.yaml"), []byte("jira:\n  api_token: \"secret-token\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		root := newRootCmd()
		root.SetArgs([]string{"config"})
		if err := root.Execute(); err != nil {
			t.Fatalf("config: %v", err)
		}
	})
	if strings.Contains(out, "secret-token") {
		t.Errorf("token leaked in config dump:\n%s", out)
	}
	if !strings.Contains(out, "REDACTED") {
		t.Errorf("token not redacted:\n%s", out)
	}
}
