package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"xynapse/internal/models"

	"gopkg.in/yaml.v3"
)

type Storage struct {
	base string
}

func NewStorage(base string) *Storage {
	if base == "~" || strings.HasPrefix(base, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(home, base[1:])
		}
	}
	return &Storage{base}
}

// Helper ensure directory exist before writing
func (s *Storage) ensureDir(dirPath string) error {
	return os.MkdirAll(dirPath, 0o755)
}

// Helper: Constructs ticket file path (e.g. ./tickets/PROJ/PROJ-123.yml)
func (s *Storage) getTicketPath(project, ticketKey string) string {
	key := strings.ToUpper(ticketKey)
	if !strings.Contains(key, "-") {
		key = fmt.Sprintf("%s-%s", strings.ToUpper(project), key)
	}
	projDir := strings.Split(key, "-")[0]
	return filepath.Join(s.base, projDir, fmt.Sprintf("%s.yml", key))
}

// Helper: Constructs sprint manifest path (e.g. ./tickets/PROJ/sprints/current.yml)
func (s *Storage) getSprintManifestPath(project string) string {
	return filepath.Join(s.base, strings.ToUpper(project), "sprints", "current.yml")
}

// GetPlanPath returns the path where a ticket's implementation plan is stored
// (e.g. <base>/plans/PROJ-123.md). A bare number is prefixed with the project.
func (s *Storage) GetPlanPath(project, ticketKey string) string {
	key := strings.ToUpper(ticketKey)
	if !strings.Contains(key, "-") {
		key = fmt.Sprintf("%s-%s", strings.ToUpper(project), key)
	}
	return filepath.Join(s.base, "plans", fmt.Sprintf("%s.md", key))
}

// HasPlan reports whether a ticket has a saved implementation plan file.
func (s *Storage) HasPlan(project, ticketKey string) bool {
	_, err := os.Stat(s.GetPlanPath(project, ticketKey))
	return err == nil
}

// PlanModTime returns the modification time of a ticket's saved plan file.
// ok is false when no plan file exists.
func (s *Storage) PlanModTime(project, ticketKey string) (modTime time.Time, ok bool) {
	fi, err := os.Stat(s.GetPlanPath(project, ticketKey))
	if err != nil {
		return time.Time{}, false
	}
	return fi.ModTime(), true
}

// WriteTicket serializes a single ticket model into local YAML.
func (s *Storage) WriteTicket(ticket *models.Ticket) error {
	filePath := s.getTicketPath(ticket.Project, ticket.Key)
	if err := s.ensureDir(filepath.Dir(filePath)); err != nil {
		return fmt.Errorf("failed to create directory for ticket: %w", err)
	}

	data, err := yaml.Marshal(ticket)
	if err != nil {
		return fmt.Errorf("failed to marshal ticket YAML: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write ticket file %s: %w", filePath, err)
	}

	return nil
}

// ReadTicket reads and unmarshals a local ticket YAML file by project and ticket key/number.
func (s *Storage) ReadTicket(project, ticketKey string) (*models.Ticket, error) {
	filePath := s.getTicketPath(project, ticketKey)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("ticket file not found at %s", filePath)
		}
		return nil, fmt.Errorf("failed to read ticket file: %w", err)
	}

	var ticket models.Ticket
	if err := yaml.Unmarshal(data, &ticket); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ticket YAML: %w", err)
	}

	return &ticket, nil
}

// WriteSprintManifest saves active sprint index and exports all fetched ticket YAMLs.
func (s *Storage) WriteSprintManifest(project string, sprintID int, sprintName string, tickets []*models.Ticket) error {
	var keys []string
	for _, ticket := range tickets {
		if err := s.WriteTicket(ticket); err != nil {
			return fmt.Errorf("failed to write ticket %s during sprint pull: %w", ticket.Key, err)
		}
		keys = append(keys, ticket.Key)
	}

	manifest := models.SprintManifest{
		Project:    strings.ToUpper(project),
		SprintID:   sprintID,
		SprintName: sprintName,
		FetchedAt:  time.Now().UTC(),
		TicketKeys: keys,
	}

	manifestPath := s.getSprintManifestPath(project)
	if err := s.ensureDir(filepath.Dir(manifestPath)); err != nil {
		return fmt.Errorf("failed to create directory for sprint manifest: %w", err)
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal sprint manifest YAML: %w", err)
	}

	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write sprint manifest file: %w", err)
	}

	return nil
}

// ReadSprintManifest loads the current sprint manifest without reading individual tickets.
func (s *Storage) ReadSprintManifest(project string) (*models.SprintManifest, error) {
	manifestPath := s.getSprintManifestPath(project)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no current sprint manifest found for project %s at %s", project, manifestPath)
		}
		return nil, fmt.Errorf("failed to read sprint manifest: %w", err)
	}

	var manifest models.SprintManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sprint manifest: %w", err)
	}

	return &manifest, nil
}

// ReadSprintTickets loads the current sprint manifest and fetches local YAML tickets without server calls.
func (s *Storage) ReadSprintTickets(project string) ([]*models.Ticket, error) {
	manifest, err := s.ReadSprintManifest(project)
	if err != nil {
		return nil, err
	}

	var tickets []*models.Ticket
	for _, key := range manifest.TicketKeys {
		ticket, err := s.ReadTicket(project, key)
		if err != nil {
			return nil, fmt.Errorf("error reading sprint ticket %s: %w", key, err)
		}
		tickets = append(tickets, ticket)
	}

	return tickets, nil
}

// Clear removes cached tickets, sprint manifests, and implementation plans.
// When project is empty, the entire storage base is wiped (including all
// plans); otherwise only that project's tickets and its plan files
// (<base>/plans/<PROJECT>-*.md) are removed. Returns the number of files
// removed.
func (s *Storage) Clear(project string) (int, error) {
	project = strings.ToUpper(project)
	removed := 0

	target := s.base
	if project != "" {
		target = filepath.Join(s.base, project)
	}

	info, err := os.Stat(target)
	if err != nil {
		if !os.IsNotExist(err) {
			return 0, fmt.Errorf("failed to stat %s: %w", target, err)
		}
	} else {
		if !info.IsDir() {
			return 0, fmt.Errorf("cache path %s is not a directory", target)
		}

		entries, err := os.ReadDir(target)
		if err != nil {
			return 0, fmt.Errorf("failed to read cache dir %s: %w", target, err)
		}
		for _, entry := range entries {
			if err := os.RemoveAll(filepath.Join(target, entry.Name())); err != nil {
				return 0, fmt.Errorf("failed to remove %s: %w", filepath.Join(target, entry.Name()), err)
			}
			removed++
		}
	}

	plansRemoved, err := s.clearPlans(project)
	if err != nil {
		return removed, err
	}
	return removed + plansRemoved, nil
}

// clearPlans removes saved implementation plans. When project is empty the
// whole plans directory is removed; otherwise only <PROJECT>-*.md files.
func (s *Storage) clearPlans(project string) (int, error) {
	plansDir := filepath.Join(s.base, "plans")
	entries, err := os.ReadDir(plansDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read plans dir %s: %w", plansDir, err)
	}

	removed := 0
	for _, entry := range entries {
		name := entry.Name()
		if project != "" && !strings.HasPrefix(name, project+"-") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(plansDir, name)); err != nil {
			return removed, fmt.Errorf("failed to remove plan %s: %w", filepath.Join(plansDir, name), err)
		}
		removed++
	}
	return removed, nil
}
