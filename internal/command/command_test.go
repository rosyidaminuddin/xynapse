package command

import (
	"strings"
	"testing"

	"xynapse/internal/models"
)

func TestParseTicketRef(t *testing.T) {
	cases := []struct {
		name           string
		ref            string
		defaultProject string
		wantProject    string
		wantNumber     string
		wantErr        bool
	}{
		{"bare number", "123", "MERADIO", "MERADIO", "123", false},
		{"full key", "MERADIO-123", "MERADIO", "MERADIO", "123", false},
		{"full key lowercase", "meradio-123", "MERADIO", "MERADIO", "123", false},
		{"browse url", "https://example.atlassian.net/browse/MERADIO-123", "MERADIO", "MERADIO", "123", false},
		{"url with trailing slash", "https://example.atlassian.net/browse/MERADIO-123/", "MERADIO", "MERADIO", "123", false},
		{"empty", "", "MERADIO", "", "", true},
		{"dash only", "-", "MERADIO", "", "", true},
		{"number only no project", "123", "", "", "", true},
		{"number only with project", "123", "MERADIO", "MERADIO", "123", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project, number, err := ParseTicketRef(tc.ref, tc.defaultProject)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseTicketRef(%q) err = %v, wantErr=%v", tc.ref, err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if project != tc.wantProject {
				t.Errorf("project = %q, want %q", project, tc.wantProject)
			}
			if number != tc.wantNumber {
				t.Errorf("number = %q, want %q", number, tc.wantNumber)
			}
		})
	}
}

func TestFilterByType(t *testing.T) {
	tickets := []*models.Ticket{
		{Key: "A-1", Type: "Story"},
		{Key: "A-2", Type: "Bug"},
		{Key: "A-3", Type: "Epic"},
		{Key: "A-4", Type: "story"},
	}

	got := filterByType(tickets, []string{"Story", "Bug"})
	if len(got) != 3 {
		t.Fatalf("filterByType len = %d, want 3", len(got))
	}
	for _, tk := range got {
		if tk.Type != "Story" && tk.Type != "Bug" && tk.Type != "story" {
			t.Errorf("unexpected ticket type %q in filtered results", tk.Type)
		}
	}
}

func TestFilterByTypeEmpty(t *testing.T) {
	tickets := []*models.Ticket{{Key: "A-1", Type: "Story"}}
	got := filterByType(tickets, nil)
	if len(got) != 1 {
		t.Errorf("empty filter should return all tickets, got %d", len(got))
	}
}

func TestTicketDossier(t *testing.T) {
	ticket := &models.Ticket{
		Key:             "PROJ-1",
		Type:            "Story",
		Status:          "In Progress",
		Assignee:        "Adin",
		Summary:         "Add clear-cache",
		DescriptionText: "A description",
	}
	d := ticketDossier(ticket)
	for _, want := range []string{"Key: PROJ-1", "Status: In Progress", "Assignee: Adin", "Summary: Add clear-cache", "A description"} {
		if !strings.Contains(d, want) {
			t.Errorf("dossier missing %q:\n%s", want, d)
		}
	}
}

func TestTicketDossierTruncatesDescription(t *testing.T) {
	long := strings.Repeat("x", 5000)
	d := ticketDossier(&models.Ticket{Key: "PROJ-1", DescriptionText: long})
	if !strings.Contains(d, "...") {
		t.Error("expected long description to be truncated")
	}
	if len(d) > 4500 {
		t.Errorf("dossier too long: %d", len(d))
	}
}
