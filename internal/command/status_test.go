package command

import (
	"strings"
	"testing"

	"xynapse/internal/storage"
)

func TestStatusShowsDefault(t *testing.T) {
	base := t.TempDir()
	writePlanFile(t, base, "PROJ-1", []byte("# plan"))
	cfg := testConfig(base)

	out := captureStdout(t, func() {
		if err := PlanStatus(cfg, "PROJ-1", ""); err != nil {
			t.Fatalf("PlanStatus: %v", err)
		}
	})
	if !strings.Contains(out, storage.PlanStatusNotStarted) {
		t.Errorf("output = %q, want %q", out, storage.PlanStatusNotStarted)
	}
}

func TestStatusMissingPlan(t *testing.T) {
	cfg := testConfig(t.TempDir())
	if err := PlanStatus(cfg, "PROJ-1", ""); err == nil {
		t.Fatal("expected error for missing plan")
	}
}

func TestStatusSet(t *testing.T) {
	base := t.TempDir()
	writePlanFile(t, base, "PROJ-1", []byte("# plan"))
	cfg := testConfig(base)

	if err := PlanStatus(cfg, "PROJ-1", storage.PlanStatusInReview); err != nil {
		t.Fatalf("PlanStatus set: %v", err)
	}
	status, ok := storage.NewStorage(base).PlanStatus("PROJ", "PROJ-1")
	if !ok || status != storage.PlanStatusInReview {
		t.Errorf("status = %q (ok=%v), want %q", status, ok, storage.PlanStatusInReview)
	}
}

func TestStatusSetInvalid(t *testing.T) {
	base := t.TempDir()
	writePlanFile(t, base, "PROJ-1", []byte("# plan"))
	cfg := testConfig(base)

	if err := PlanStatus(cfg, "PROJ-1", "bogus"); err == nil {
		t.Fatal("expected error for invalid status")
	}
}
