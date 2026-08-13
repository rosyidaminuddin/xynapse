package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DriveState records which pipeline steps the drive command has completed for
// a ticket, keyed by step name ("branch", "plan", "implement", "test", "lint",
// "finalize", "ticket"). It lives at <base>/drive/<PROJECT>/<KEY>.yml so drive
// is resumable across invocations.
type DriveState struct {
	// Steps maps a step name to the time it completed.
	Steps map[string]time.Time `yaml:"steps"`
	// Failures records per-step failure messages (e.g. a failed test run).
	// A step in Failures can be re-run; --force skips the gate it represents.
	Failures map[string]string `yaml:"failures"`
}

func newDriveState() DriveState {
	return DriveState{
		Steps:    map[string]time.Time{},
		Failures: map[string]string{},
	}
}

// getDriveStatePath returns the drive state file path for a ticket.
func (s *Storage) getDriveStatePath(project, ticketKey string) string {
	key := strings.ToUpper(ticketKey)
	return filepath.Join(s.base, "drive", strings.ToUpper(project), fmt.Sprintf("%s.yml", key))
}

// ReadDriveState loads a ticket's drive state. A missing state file yields an
// empty state with no error.
func (s *Storage) ReadDriveState(project, ticketKey string) (DriveState, error) {
	path := s.getDriveStatePath(project, ticketKey)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newDriveState(), nil
		}
		return newDriveState(), fmt.Errorf("failed to read drive state %s: %w", path, err)
	}

	var st DriveState
	if err := yaml.Unmarshal(data, &st); err != nil {
		return newDriveState(), fmt.Errorf("failed to parse drive state %s: %w", path, err)
	}
	if st.Steps == nil {
		st.Steps = map[string]time.Time{}
	}
	if st.Failures == nil {
		st.Failures = map[string]string{}
	}
	return st, nil
}

// WriteDriveState persists a ticket's drive state.
func (s *Storage) WriteDriveState(project, ticketKey string, st DriveState) error {
	path := s.getDriveStatePath(project, ticketKey)
	if err := s.ensureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("failed to create drive state dir: %w", err)
	}
	data, err := yaml.Marshal(st)
	if err != nil {
		return fmt.Errorf("failed to marshal drive state: %w", err)
	}
	if err := writeFileAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write drive state %s: %w", path, err)
	}
	return nil
}

// SetDriveStep marks a step as completed at the current time.
func (s *Storage) SetDriveStep(project, ticketKey, step string) error {
	st, err := s.ReadDriveState(project, ticketKey)
	if err != nil {
		return err
	}
	st.Steps[step] = time.Now().UTC()
	return s.WriteDriveState(project, ticketKey, st)
}

// DriveStepDone reports whether a step has completed.
func (s *Storage) DriveStepDone(project, ticketKey, step string) (bool, error) {
	st, err := s.ReadDriveState(project, ticketKey)
	if err != nil {
		return false, err
	}
	_, ok := st.Steps[step]
	return ok, nil
}

// SetDriveFailure records a failure message for a step.
func (s *Storage) SetDriveFailure(project, ticketKey, step, msg string) error {
	st, err := s.ReadDriveState(project, ticketKey)
	if err != nil {
		return err
	}
	st.Failures[step] = msg
	return s.WriteDriveState(project, ticketKey, st)
}

// ClearDriveFailure removes a recorded failure for a step.
func (s *Storage) ClearDriveFailure(project, ticketKey, step string) error {
	st, err := s.ReadDriveState(project, ticketKey)
	if err != nil {
		return err
	}
	delete(st.Failures, step)
	return s.WriteDriveState(project, ticketKey, st)
}

// DriveStepFailed reports whether a step has a recorded failure.
func (s *Storage) DriveStepFailed(project, ticketKey, step string) (string, bool) {
	st, err := s.ReadDriveState(project, ticketKey)
	if err != nil {
		return "", false
	}
	msg, ok := st.Failures[step]
	return msg, ok
}
