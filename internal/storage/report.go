package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetReportPath returns the path where a ticket's implement report is stored
// (e.g. <base>/plans/PROJ-123.report.md). The report carries the agent's
// acceptance-criteria results so drive can gate finalize on them.
func (s *Storage) GetReportPath(project, ticketKey string) string {
	key := strings.ToUpper(ticketKey)
	if !strings.Contains(key, "-") {
		key = fmt.Sprintf("%s-%s", strings.ToUpper(project), key)
	}
	return filepath.Join(s.base, "plans", fmt.Sprintf("%s.report.md", key))
}

// HasReport reports whether a ticket has a saved implement report.
func (s *Storage) HasReport(project, ticketKey string) bool {
	_, err := os.Stat(s.GetReportPath(project, ticketKey))
	return err == nil
}

// WriteReport persists a ticket's implement report atomically.
func (s *Storage) WriteReport(project, ticketKey, report string) error {
	path := s.GetReportPath(project, ticketKey)
	if err := s.ensureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("failed to create report dir: %w", err)
	}
	if err := writeFileAtomic(path, []byte(report), 0o644); err != nil {
		return fmt.Errorf("failed to write report %s: %w", path, err)
	}
	return nil
}

// ReadReport loads a ticket's implement report. ok is false when no report
// file exists.
func (s *Storage) ReadReport(project, ticketKey string) (body string, ok bool) {
	data, err := os.ReadFile(s.GetReportPath(project, ticketKey))
	if err != nil {
		return "", false
	}
	return string(data), true
}