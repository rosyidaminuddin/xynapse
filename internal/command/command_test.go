package command

import (
	"io"
	"os"
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

func TestRenderMDPassthroughToBuffer(t *testing.T) {
	md := "# Plan\n\n- step one\n- step two\n"
	var buf strings.Builder
	if err := RenderMD(&buf, md); err != nil {
		t.Fatalf("RenderMD err = %v", err)
	}
	if buf.String() != md {
		t.Errorf("non-terminal writer should pass through raw markdown, got:\n%q", buf.String())
	}
}

func TestRenderMDPassthroughToFile(t *testing.T) {
	md := "# Plan\n"
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := RenderMD(f, md); err != nil {
		t.Fatalf("RenderMD err = %v", err)
	}
	got, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != md {
		t.Errorf("file writer should pass through raw markdown, got:\n%q", string(got))
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	f()
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestPrintSprintTicketsPlanStatus(t *testing.T) {
	views := []SprintTicket{
		{Ticket: &models.Ticket{Key: "PROJ-1", Status: "Open", Assignee: "A", Summary: "S1"}, Plan: true},
		{Ticket: &models.Ticket{Key: "PROJ-2", Status: "Done", Assignee: "B", Summary: "S2"}, Plan: false},
	}

	table := captureStdout(t, func() {
		if err := printSprintTickets("", views); err != nil {
			t.Errorf("printSprintTickets: %v", err)
		}
	})
	for _, want := range []string{"KEY", "PLAN", "yes", "no"} {
		if !strings.Contains(table, want) {
			t.Errorf("table missing %q:\n%s", want, table)
		}
	}

	js := captureStdout(t, func() {
		if err := printSprintTickets("json", views); err != nil {
			t.Errorf("printSprintTickets json: %v", err)
		}
	})
	for _, want := range []string{`"Key": "PROJ-1"`, `"plan": true`, `"plan": false`} {
		if !strings.Contains(js, want) {
			t.Errorf("json missing %q:\n%s", want, js)
		}
	}
}
