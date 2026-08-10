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
	ID                    string    `yaml:"id"`
	Key                   string    `yaml:"key"`
	Project               string    `yaml:"project"`
	Type                  string    `yaml:"type,omitempty"`
	Summary               string    `yaml:"summary"`
	Status                string    `yaml:"status"`
	Assignee              string    `yaml:"assignee"`
	Description           string    `yaml:"description,omitempty"`
	DescriptionText       string    `yaml:"description_text,omitempty"`
	AcceptanceCriteria    string    `yaml:"acceptance_criteria,omitempty"`
	AcceptanceCriteriaText string   `yaml:"acceptance_criteria_text,omitempty"`
	FetchedAt             time.Time `yaml:"fetched_at"`
	UpdatedAt             time.Time `yaml:"updated_at"`
}

type JiraRawIssue struct {
	ID     string          `json:"id"`
	Key    string          `json:"key"`
	Fields json.RawMessage `json:"fields"`
}

// jiraFields mirrors the standard Jira issue fields returned by the API. The
// full raw fields blob is also decoded into a map so configured custom fields
// can be looked up by id.
type jiraFields struct {
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
}

// MapRawToTicket converts a raw Jira issue into a local ticket. When
// acceptanceCriteriaField is set, that custom field (an ADF document or plain
// string) is captured as both raw JSON and flattened text.
func MapRawToTicket(raw *JiraRawIssue, acceptanceCriteriaField string) *Ticket {
	var fields jiraFields
	if len(raw.Fields) > 0 {
		_ = json.Unmarshal(raw.Fields, &fields)
	}

	assignee := "Unassigned"
	if fields.Assignee.DisplayName != "" {
		assignee = fields.Assignee.DisplayName
	}

	// Parse string into time.Time
	layout := "2006-01-02T15:04:05.999Z0700"
	updatedAt, err := time.Parse(layout, fields.Updated)
	if err != nil {
		updatedAt = time.Now().UTC()
	}

	description := ""
	descriptionText := ""
	if len(fields.Description) > 0 && string(fields.Description) != "null" {
		description = string(fields.Description)
		descriptionText = flattenADF(fields.Description)
	}

	acceptanceCriteria := ""
	acceptanceCriteriaText := ""
	if acceptanceCriteriaField != "" {
		if rawFields, ok := rawFieldsMap(raw); ok {
			if value, found := rawFields[acceptanceCriteriaField]; found {
				acceptanceCriteria, acceptanceCriteriaText = extractField(value)
			}
		}
	}

	return &Ticket{
		ID:                     raw.ID,
		Key:                    raw.Key,
		Project:                fields.Project.Key,
		Type:                   fields.Issuetype.Name,
		Summary:                fields.Summary,
		Status:                 fields.Status.Name,
		Assignee:               assignee,
		Description:            description,
		DescriptionText:        descriptionText,
		AcceptanceCriteria:     acceptanceCriteria,
		AcceptanceCriteriaText: acceptanceCriteriaText,
		FetchedAt:              time.Now().UTC(),
		UpdatedAt:              updatedAt,
	}
}

// rawFieldsMap decodes the raw fields blob into a map keyed by field id.
func rawFieldsMap(raw *JiraRawIssue) (map[string]json.RawMessage, bool) {
	if len(raw.Fields) == 0 || string(raw.Fields) == "null" {
		return nil, false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw.Fields, &m); err != nil {
		return nil, false
	}
	return m, true
}

// extractField captures a custom field value as raw JSON plus flattened text.
// ADF documents are flattened like descriptions; plain strings pass through.
func extractField(raw json.RawMessage) (string, string) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return string(raw), strings.TrimSpace(s)
	}
	return string(raw), flattenADF(raw)
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
		"paragraph":   true,
		"heading":     true,
		"bulletList":  true,
		"orderedList": true,
		"listItem":    true,
		"blockquote":  true,
		"codeBlock":   true,
		"rule":        true,
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
