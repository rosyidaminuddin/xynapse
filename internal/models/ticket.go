package models

import (
	"fmt"
	"time"
)

// SprintManifest records the active sprint tickets metadata and index[cite: 1]
type SprintManifest struct {
	Project    string    `yaml:"project"`
	FetchedAt  time.Time `yaml:"fetched_at"`
	TicketKeys []string  `yaml:"ticket_keys"`
}

type Ticket struct {
	ID          string    `yaml:"id"`
	Key         string    `yaml:"key"`
	Project     string    `yaml:"project"`
	Summary     string    `yaml:"summary"`
	Status      string    `yaml:"status"`
	Assignee    string    `yaml:"assignee"`
	Description string    `yaml:"description,omitempty"`
	FetchedAt   time.Time `yaml:"fetched_at"`
	UpdatedAt   time.Time `yaml:"updated_at"`
}

type JiraRawIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Project struct {
			Key string `json:"key"`
		} `json:"project"`
		Status struct {
			Name string `json:"name"`
		} `json:"status"`
		Assignee struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Updated string `json:"updated"`
	} `json:"fields"`
}

func MapRawToTicket(raw *JiraRawIssue) *Ticket {
	assignee := "Unassigned"

	if raw.Fields.Assignee.DisplayName != "" {
		assignee = raw.Fields.Assignee.DisplayName
	}

	// Parse string into time.Time
	updatedAt, err := time.Parse(time.RFC3339, raw.Fields.Updated)
	if err != nil {
		fmt.Printf("Error parsing updated time: %v\n", err)
		updatedAt = time.Now().UTC()
	}

	return &Ticket{
		ID:        raw.ID,
		Key:       raw.Key,
		Project:   raw.Fields.Project.Key,
		Summary:   raw.Fields.Summary,
		Status:    raw.Fields.Status.Name,
		Assignee:  assignee,
		FetchedAt: time.Now().UTC(),
		UpdatedAt: updatedAt,
	}
}
