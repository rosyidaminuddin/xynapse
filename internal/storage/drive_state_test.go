package storage

import "testing"

func TestDriveStepRoundTrip(t *testing.T) {
	s := NewStorage(t.TempDir())

	done, err := s.DriveStepDone("PROJ", "PROJ-1", "branch")
	if err != nil {
		t.Fatalf("DriveStepDone: %v", err)
	}
	if done {
		t.Error("expected branch step to be pending initially")
	}

	if err := s.SetDriveStep("PROJ", "PROJ-1", "branch"); err != nil {
		t.Fatalf("SetDriveStep: %v", err)
	}
	done, err = s.DriveStepDone("PROJ", "PROJ-1", "branch")
	if err != nil {
		t.Fatalf("DriveStepDone: %v", err)
	}
	if !done {
		t.Error("expected branch step to be done after SetDriveStep")
	}

	// Steps are per-ticket: PROJ-2 stays pending.
	if done, _ := s.DriveStepDone("PROJ", "PROJ-2", "branch"); done {
		t.Error("PROJ-2 branch step should not be done")
	}
}

func TestDriveStepKeysNormalized(t *testing.T) {
	s := NewStorage(t.TempDir())
	if err := s.SetDriveStep("proj", "proj-1", "test"); err != nil {
		t.Fatalf("SetDriveStep: %v", err)
	}
	// Lowercase key resolves to the same state file as uppercase.
	done, err := s.DriveStepDone("PROJ", "PROJ-1", "test")
	if err != nil {
		t.Fatalf("DriveStepDone: %v", err)
	}
	if !done {
		t.Error("expected step to be found with normalized keys")
	}
}

func TestDriveFailureRoundTrip(t *testing.T) {
	s := NewStorage(t.TempDir())

	if _, ok := s.DriveStepFailed("PROJ", "PROJ-1", "test"); ok {
		t.Error("expected no failure initially")
	}

	if err := s.SetDriveFailure("PROJ", "PROJ-1", "test", "go test failed"); err != nil {
		t.Fatalf("SetDriveFailure: %v", err)
	}
	msg, ok := s.DriveStepFailed("PROJ", "PROJ-1", "test")
	if !ok {
		t.Fatal("expected recorded failure")
	}
	if msg != "go test failed" {
		t.Errorf("failure msg = %q", msg)
	}

	if err := s.ClearDriveFailure("PROJ", "PROJ-1", "test"); err != nil {
		t.Fatalf("ClearDriveFailure: %v", err)
	}
	if _, ok := s.DriveStepFailed("PROJ", "PROJ-1", "test"); ok {
		t.Error("expected failure cleared")
	}
}

func TestDriveStepAndFailureIndependent(t *testing.T) {
	s := NewStorage(t.TempDir())
	if err := s.SetDriveStep("PROJ", "PROJ-1", "test"); err != nil {
		t.Fatalf("SetDriveStep: %v", err)
	}
	if err := s.SetDriveFailure("PROJ", "PROJ-1", "test", "failed"); err != nil {
		t.Fatalf("SetDriveFailure: %v", err)
	}
	// A completed step can still carry a failure gate (e.g. --force bypass).
	if done, _ := s.DriveStepDone("PROJ", "PROJ-1", "test"); !done {
		t.Error("expected step done")
	}
	if _, ok := s.DriveStepFailed("PROJ", "PROJ-1", "test"); !ok {
		t.Error("expected failure retained")
	}
}

func TestDriveStateMissingTicket(t *testing.T) {
	s := NewStorage(t.TempDir())
	st, err := s.ReadDriveState("PROJ", "PROJ-99")
	if err != nil {
		t.Fatalf("ReadDriveState: %v", err)
	}
	if len(st.Steps) != 0 || len(st.Failures) != 0 {
		t.Errorf("expected empty state, got %+v", st)
	}
}
