package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadPlanDefaultsToNotStarted(t *testing.T) {
	s := NewStorage(t.TempDir())
	body := "# plan\n\n- step one\n"
	writePlan(t, s, "PROJ", "PROJ-1", body)

	gotBody, status, ok := s.ReadPlan("PROJ", "PROJ-1")
	if !ok {
		t.Fatal("expected plan to exist")
	}
	if status != PlanStatusNotStarted {
		t.Errorf("status = %q, want %q", status, PlanStatusNotStarted)
	}
	if gotBody != body {
		t.Errorf("body = %q, want %q", gotBody, body)
	}
}

func TestReadPlanMissing(t *testing.T) {
	s := NewStorage(t.TempDir())
	if _, _, ok := s.ReadPlan("PROJ", "PROJ-9"); ok {
		t.Error("expected ok=false for missing plan")
	}
	if _, ok := s.PlanStatus("PROJ", "PROJ-9"); ok {
		t.Error("expected ok=false for missing plan status")
	}
}

func TestSetPlanStatusRoundTrip(t *testing.T) {
	s := NewStorage(t.TempDir())
	body := "# plan\n\n- step one\n"
	writePlan(t, s, "PROJ", "PROJ-1", body)

	if err := s.SetPlanStatus("PROJ", "PROJ-1", PlanStatusInProgress); err != nil {
		t.Fatalf("SetPlanStatus: %v", err)
	}

	gotBody, status, ok := s.ReadPlan("PROJ", "PROJ-1")
	if !ok {
		t.Fatal("expected plan to exist")
	}
	if status != PlanStatusInProgress {
		t.Errorf("status = %q, want %q", status, PlanStatusInProgress)
	}
	if gotBody != body {
		t.Errorf("body changed after status update:\n%q", gotBody)
	}

	data, err := os.ReadFile(filepath.Join(s.base, "plans", "PROJ-1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "---\nstatus: in progress\n---\n") {
		t.Errorf("plan file should have status frontmatter, got:\n%s", string(data))
	}
}

func TestSetPlanStatusOverwritesExisting(t *testing.T) {
	s := NewStorage(t.TempDir())
	writePlan(t, s, "PROJ", "PROJ-1", "---\nstatus: in review\ncustom: x\n---\n# plan\n")

	if err := s.SetPlanStatus("PROJ", "PROJ-1", PlanStatusDone); err != nil {
		t.Fatalf("SetPlanStatus: %v", err)
	}

	_, status, _ := s.ReadPlan("PROJ", "PROJ-1")
	if status != PlanStatusDone {
		t.Errorf("status = %q, want %q", status, PlanStatusDone)
	}

	data, _ := os.ReadFile(filepath.Join(s.base, "plans", "PROJ-1.md"))
	if !strings.Contains(string(data), "custom: x") {
		t.Errorf("existing frontmatter keys should be preserved, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), "# plan") {
		t.Errorf("body should be preserved, got:\n%s", string(data))
	}
}

func TestSetPlanStatusErrors(t *testing.T) {
	s := NewStorage(t.TempDir())
	if err := s.SetPlanStatus("PROJ", "PROJ-9", PlanStatusDone); err == nil {
		t.Error("expected error for missing plan")
	}

	writePlan(t, s, "PROJ", "PROJ-1", "# plan\n")
	if err := s.SetPlanStatus("PROJ", "PROJ-1", "bogus"); err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestValidPlanStatus(t *testing.T) {
	for _, s := range []string{PlanStatusNotStarted, PlanStatusInProgress, PlanStatusInReview, PlanStatusDone} {
		if !ValidPlanStatus(s) {
			t.Errorf("ValidPlanStatus(%q) = false", s)
		}
	}
	if ValidPlanStatus("bogus") {
		t.Error("ValidPlanStatus(bogus) = true")
	}
}

func TestSetPlanStatusNormalizesSeparators(t *testing.T) {
	s := NewStorage(t.TempDir())
	writePlan(t, s, "PROJ", "PROJ-1", "# plan\n")

	if err := s.SetPlanStatus("PROJ", "PROJ-1", "in_review"); err != nil {
		t.Fatalf("SetPlanStatus(in_review): %v", err)
	}
	if status, _ := s.PlanStatus("PROJ", "PROJ-1"); status != PlanStatusInReview {
		t.Errorf("status = %q, want %q", status, PlanStatusInReview)
	}

	if err := s.SetPlanStatus("PROJ", "PROJ-1", "in-progress"); err != nil {
		t.Fatalf("SetPlanStatus(in-progress): %v", err)
	}
	if status, _ := s.PlanStatus("PROJ", "PROJ-1"); status != PlanStatusInProgress {
		t.Errorf("status = %q, want %q", status, PlanStatusInProgress)
	}
}
