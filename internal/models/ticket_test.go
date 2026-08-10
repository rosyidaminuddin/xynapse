package models

import (
	"encoding/json"
	"testing"
)

func TestMapRawToTicket(t *testing.T) {
	raw := `{
		"id": "10000",
		"key": "MERADIO-999",
		"fields": {
			"summary": "A summary",
			"description": {
				"type": "doc",
				"content": [
					{"type": "paragraph", "content": [
						{"type": "text", "text": "Hello "},
						{"type": "text", "text": "world"}
					]},
					{"type": "bulletList", "content": [
						{"type": "listItem", "content": [
							{"type": "paragraph", "content": [
								{"type": "text", "text": "item one"}
							]}
						]}
					]}
				]
			},
			"issuetype": {"name": "Story"},
			"project": {"key": "MERADIO"},
			"status": {"name": "In Progress"},
			"assignee": {"displayName": "Adin"},
			"updated": "2026-08-09T10:00:00.000+0000"
		}
	}`

	var issue JiraRawIssue
	if err := json.Unmarshal([]byte(raw), &issue); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	ticket := MapRawToTicket(&issue, "")

	if ticket.Key != "MERADIO-999" {
		t.Errorf("Key = %q, want %q", ticket.Key, "MERADIO-999")
	}
	if ticket.Type != "Story" {
		t.Errorf("Type = %q, want %q", ticket.Type, "Story")
	}
	if ticket.Status != "In Progress" {
		t.Errorf("Status = %q, want %q", ticket.Status, "In Progress")
	}
	if ticket.Assignee != "Adin" {
		t.Errorf("Assignee = %q, want %q", ticket.Assignee, "Adin")
	}
	if ticket.Description == "" {
		t.Error("Description should hold the raw ADF JSON")
	}
	if ticket.DescriptionText != "Hello world\nitem one" {
		t.Errorf("DescriptionText = %q, want %q", ticket.DescriptionText, "Hello world\nitem one")
	}
}

func TestMapRawToTicketNullDescription(t *testing.T) {
	raw := `{
		"id": "10001",
		"key": "MERADIO-1000",
		"fields": {
			"summary": "No desc",
			"description": null,
			"issuetype": {"name": "Bug"},
			"project": {"key": "MERADIO"},
			"status": {"name": "Open"},
			"updated": "2026-08-09T10:00:00.000+0000"
		}
	}`

	var issue JiraRawIssue
	if err := json.Unmarshal([]byte(raw), &issue); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	ticket := MapRawToTicket(&issue, "")

	if ticket.Description != "" {
		t.Errorf("Description = %q, want empty", ticket.Description)
	}
	if ticket.DescriptionText != "" {
		t.Errorf("DescriptionText = %q, want empty", ticket.DescriptionText)
	}
	if ticket.Assignee != "Unassigned" {
		t.Errorf("Assignee = %q, want %q", ticket.Assignee, "Unassigned")
	}
}

func TestMapRawToTicketMissingDescription(t *testing.T) {
	raw := `{
		"id": "10002",
		"key": "MERADIO-1002",
		"fields": {
			"summary": "No desc field",
			"issuetype": {"name": "Epic"},
			"project": {"key": "MERADIO"},
			"status": {"name": "Done"},
			"updated": "2026-08-09T10:00:00.000+0000"
		}
	}`

	var issue JiraRawIssue
	if err := json.Unmarshal([]byte(raw), &issue); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	ticket := MapRawToTicket(&issue, "")
	if ticket.DescriptionText != "" {
		t.Errorf("DescriptionText = %q, want empty", ticket.DescriptionText)
	}
}

func TestMapRawToTicketAcceptanceCriteriaADF(t *testing.T) {
	raw := `{
		"id": "10003",
		"key": "MERADIO-1003",
		"fields": {
			"summary": "AC story",
			"description": null,
			"customfield_10001": {
				"type": "doc",
				"content": [
					{"type": "bulletList", "content": [
						{"type": "listItem", "content": [
							{"type": "paragraph", "content": [
								{"type": "text", "text": "Acceptance one"}
							]}
						]},
						{"type": "listItem", "content": [
							{"type": "paragraph", "content": [
								{"type": "text", "text": "Acceptance two"}
							]}
						]}
					]}
				]
			},
			"issuetype": {"name": "Story"},
			"project": {"key": "MERADIO"},
			"status": {"name": "To Do"},
			"updated": "2026-08-09T10:00:00.000+0000"
		}
	}`

	var issue JiraRawIssue
	if err := json.Unmarshal([]byte(raw), &issue); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	ticket := MapRawToTicket(&issue, "customfield_10001")

	if ticket.AcceptanceCriteria == "" {
		t.Error("AcceptanceCriteria should hold the raw ADF JSON")
	}
	if ticket.AcceptanceCriteriaText != "Acceptance one\n\nAcceptance two" {
		t.Errorf("AcceptanceCriteriaText = %q, want %q", ticket.AcceptanceCriteriaText, "Acceptance one\n\nAcceptance two")
	}
}

func TestMapRawToTicketAcceptanceCriteriaPlainString(t *testing.T) {
	raw := `{
		"id": "10004",
		"key": "MERADIO-1004",
		"fields": {
			"summary": "Plain AC",
			"description": null,
			"customfield_20002": "Must handle empty input",
			"issuetype": {"name": "Bug"},
			"project": {"key": "MERADIO"},
			"status": {"name": "Open"},
			"updated": "2026-08-09T10:00:00.000+0000"
		}
	}`

	var issue JiraRawIssue
	if err := json.Unmarshal([]byte(raw), &issue); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	ticket := MapRawToTicket(&issue, "customfield_20002")

	if ticket.AcceptanceCriteria == "" {
		t.Error("AcceptanceCriteria should hold the raw field value")
	}
	if ticket.AcceptanceCriteriaText != "Must handle empty input" {
		t.Errorf("AcceptanceCriteriaText = %q, want %q", ticket.AcceptanceCriteriaText, "Must handle empty input")
	}
}

func TestMapRawToTicketAcceptanceCriteriaMissingOrNull(t *testing.T) {
	raw := `{
		"id": "10005",
		"key": "MERADIO-1005",
		"fields": {
			"summary": "No AC",
			"description": null,
			"customfield_10001": null,
			"issuetype": {"name": "Story"},
			"project": {"key": "MERADIO"},
			"status": {"name": "To Do"},
			"updated": "2026-08-09T10:00:00.000+0000"
		}
	}`

	var issue JiraRawIssue
	if err := json.Unmarshal([]byte(raw), &issue); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	ticket := MapRawToTicket(&issue, "customfield_10001")

	if ticket.AcceptanceCriteria != "" {
		t.Errorf("AcceptanceCriteria = %q, want empty", ticket.AcceptanceCriteria)
	}
	if ticket.AcceptanceCriteriaText != "" {
		t.Errorf("AcceptanceCriteriaText = %q, want empty", ticket.AcceptanceCriteriaText)
	}

	// A configured field id that is absent from the issue stays empty too.
	ticket = MapRawToTicket(&issue, "customfield_99999")
	if ticket.AcceptanceCriteriaText != "" {
		t.Errorf("AcceptanceCriteriaText = %q, want empty for absent field", ticket.AcceptanceCriteriaText)
	}
}
