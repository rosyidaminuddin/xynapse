package models

import (
	"encoding/json"
	"strings"
	"time"
)

// SprintManifest records the active sprint tickets metadata and index[cite: 1]
type SprintManifest struct {
	Project    string    `yaml:"project"`
	SprintID   int       `yaml:"sprint_id,omitempty"`
	SprintName string    `yaml:"sprint_name,omitempty"`
	FetchedAt  time.Time `yaml:"fetched_at"`
	TicketKeys []string  `yaml:"ticket_keys"`
}

// Sprint is the agile sprint metadata returned by the Jira Agile API.
type Sprint struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type Ticket struct {
	ID              string    `yaml:"id"`
	Key             string    `yaml:"key"`
	Project         string    `yaml:"project"`
	Type            string    `yaml:"type,omitempty"`
	Summary         string    `yaml:"summary"`
	Status          string    `yaml:"status"`
	Assignee        string    `yaml:"assignee"`
	Description     string    `yaml:"description,omitempty"`
	DescriptionText string    `yaml:"description_text,omitempty"`
	FetchedAt       time.Time `yaml:"fetched_at"`
	UpdatedAt       time.Time `yaml:"updated_at"`
}

type JiraRawIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Issuetype   struct {
			Name string `json:"name"`
		} `json:"issuetype"`
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
	layout := "2006-01-02T15:04:05.999Z0700"
	updatedAt, err := time.Parse(layout, raw.Fields.Updated)
	if err != nil {
		updatedAt = time.Now().UTC()
	}

	description := ""
	descriptionText := ""
	if len(raw.Fields.Description) > 0 && string(raw.Fields.Description) != "null" {
		description = string(raw.Fields.Description)
		descriptionText = flattenADF(raw.Fields.Description)
	}

	return &Ticket{
		ID:              raw.ID,
		Key:             raw.Key,
		Project:         raw.Fields.Project.Key,
		Type:            raw.Fields.Issuetype.Name,
		Summary:         raw.Fields.Summary,
		Status:          raw.Fields.Status.Name,
		Assignee:        assignee,
		Description:     description,
		DescriptionText: descriptionText,
		FetchedAt:       time.Now().UTC(),
		UpdatedAt:       updatedAt,
	}
}

// adfContent mirrors a node of the Atlassian Document Format used by Jira descriptions.
type adfContent struct {
	Type    string       `json:"type"`
	Text    string       `json:"text"`
	Content []adfContent `json:"content"`
}

// flattenADF walks an ADF document and concatenates text nodes into plain text,
// separating block-level nodes (paragraphs, list items, etc.) with newlines.
func flattenADF(raw json.RawMessage) string {
	var doc adfContent
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}

	blockTypes := map[string]bool{
		"paragraph":  true,
		"heading":    true,
		"bulletList": true,
		"orderedList": true,
		"listItem":   true,
		"blockquote": true,
		"codeBlock":  true,
		"rule":       true,
	}

	var sb strings.Builder
	var walk func(nodes []adfContent)
	walk = func(nodes []adfContent) {
		for _, n := range nodes {
			if n.Type == "text" && n.Text != "" {
				sb.WriteString(n.Text)
			}
			if len(n.Content) > 0 {
				walk(n.Content)
			}
			if blockTypes[n.Type] {
				sb.WriteString("\n")
			}
		}
	}
	walk(doc.Content)

	return strings.TrimSpace(sb.String())
}
