package storage

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Supported plan lifecycle statuses stored in a plan file's frontmatter.
const (
	PlanStatusNotStarted = "not started"
	PlanStatusInProgress = "in progress"
	PlanStatusInReview   = "in review"
	PlanStatusDone       = "done"
)

// ValidPlanStatus reports whether s is one of the supported plan statuses.
func ValidPlanStatus(s string) bool {
	switch normalizeStatus(s) {
	case PlanStatusNotStarted, PlanStatusInProgress, PlanStatusInReview, PlanStatusDone:
		return true
	}
	return false
}

// normalizeStatus maps CLI-friendly separators onto the spaced status values,
// so "in_review", "in-review", and "in review" all refer to the same status.
func normalizeStatus(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "_", " "), "-", " ")
}

// ReadPlan reads a ticket's plan file, separating its YAML frontmatter (if
// any) from the markdown body. ok is false when no plan file exists. Files
// without a frontmatter block default to PlanStatusNotStarted.
func (s *Storage) ReadPlan(project, ticketKey string) (body, status string, ok bool) {
	data, err := os.ReadFile(s.GetPlanPath(project, ticketKey))
	if err != nil {
		return "", "", false
	}
	body, meta := splitFrontmatter(string(data))
	status = PlanStatusNotStarted
	if v, ok := meta["status"]; ok && v != "" {
		status = v
	}
	return body, status, true
}

// PlanStatus returns the status stored in a ticket's plan frontmatter. ok is
// false when the plan file does not exist.
func (s *Storage) PlanStatus(project, ticketKey string) (status string, ok bool) {
	_, status, ok = s.ReadPlan(project, ticketKey)
	return status, ok
}

// SetPlanStatus updates the status in a ticket's plan frontmatter, preserving
// the markdown body exactly. An error is returned if the plan does not exist
// or the status is not one of the supported values.
func (s *Storage) SetPlanStatus(project, ticketKey, status string) error {
	if !ValidPlanStatus(status) {
		return fmt.Errorf("invalid plan status %q (valid: %s, %s, %s, %s)",
			status, PlanStatusNotStarted, PlanStatusInProgress, PlanStatusInReview, PlanStatusDone)
	}

	path := s.GetPlanPath(project, ticketKey)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("no plan found at %s: %w", path, err)
	}

	body, meta := splitFrontmatter(string(data))
	meta["status"] = normalizeStatus(status)
	return s.writePlan(path, meta, body)
}

// RewritePlanBody replaces a ticket's plan markdown body while preserving its
// frontmatter (including the status) exactly.
func (s *Storage) RewritePlanBody(project, ticketKey, body string) error {
	path := s.GetPlanPath(project, ticketKey)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("no plan found at %s: %w", path, err)
	}
	_, meta := splitFrontmatter(string(data))
	return s.writePlan(path, meta, body)
}

func (s *Storage) writePlan(path string, meta map[string]string, body string) error {
	front, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal plan frontmatter: %w", err)
	}

	var out strings.Builder
	out.WriteString("---\n")
	out.Write(front)
	out.WriteString("---\n")
	out.WriteString(body)
	return writeFileAtomic(path, []byte(out.String()), 0o644)
}

// splitFrontmatter splits plan content into a markdown body and a metadata
// map parsed from a leading `---` delimited YAML block. Files without such a
// block return the raw content as body and an empty map.
func splitFrontmatter(content string) (body string, meta map[string]string) {
	meta = map[string]string{}
	if !strings.HasPrefix(content, "---\n") {
		return content, meta
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return content, meta
	}
	if err := yaml.Unmarshal([]byte(rest[:end]), &meta); err != nil {
		return content, meta
	}
	return rest[end+len("\n---\n"):], meta
}
