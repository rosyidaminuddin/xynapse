package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"xynapse/internal/config"
)

func TestShowPlanMissing(t *testing.T) {
	base := t.TempDir()
	cfg := testConfig(base)

	err := ShowPlan(cfg, "PROJ-1")
	if err == nil {
		t.Fatal("expected error for missing plan")
	}
	if !strings.Contains(err.Error(), "xynapse plan PROJ-1") {
		t.Errorf("error should hint at running plan first, got %v", err)
	}
}

func TestShowPlanEmpty(t *testing.T) {
	base := t.TempDir()
	writePlanFile(t, base, "PROJ-1", nil)
	cfg := testConfig(base)

	err := ShowPlan(cfg, "PROJ-1")
	if err == nil {
		t.Fatal("expected error for empty plan")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("error should mention empty plan, got %v", err)
	}
}

func TestShowPlanRenders(t *testing.T) {
	base := t.TempDir()
	plan := "# Plan\n\n- step one\n- step two\n"
	writePlanFile(t, base, "PROJ-1", []byte(plan))
	cfg := testConfig(base)

	out := captureStdout(t, func() {
		if err := ShowPlan(cfg, "1"); err != nil { // bare number uses default project
			t.Errorf("ShowPlan: %v", err)
		}
	})
	if out != plan {
		t.Errorf("ShowPlan output = %q, want %q", out, plan)
	}
}

func testConfig(base string) *config.Config {
	return &config.Config{
		Defaults: config.Defaults{Project: "PROJ"},
		Storage:  config.StorageConfig{Base: base},
	}
}

func writePlanFile(t *testing.T, base, key string, content []byte) {
	t.Helper()
	path := filepath.Join(base, "plans", key+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
