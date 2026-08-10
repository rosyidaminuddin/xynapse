package command

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xynapse/internal/config"
	"xynapse/internal/storage"
)

const (
	usersJaneDoe = `[
		{"accountId":"5b10a2844c20165700ede21g","displayName":"Jane Doe","emailAddress":"jane@corp.com","active":true}
	]`

	usersAmbiguous = `[
		{"accountId":"5b10a2844c20165700ede21g","displayName":"Jane Doe","emailAddress":"jane@corp.com","active":true},
		{"accountId":"5b10a2844c20165700ede22h","displayName":"Jane Roe","emailAddress":"jane@acme.com","active":true}
	]`
)

// assigneeServer mocks the Jira API for assignee tests, serving the given user
// search payload. It records PUT bodies (assignee updates) and serves the
// issue for the show/refresh flows.
func assigneeServer(t *testing.T, usersJSON string) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var put []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/user/search"):
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, usersJSON)
		case strings.HasSuffix(r.URL.Path, "/issue/PROJ-1") && r.Method == http.MethodPut:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("bad request body: %v", err)
			}
			put = append(put, body)
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/issue/PROJ-1"):
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":"100","key":"PROJ-1","fields":{
				"summary":"Do stuff","description":{"type":"doc","version":1,"content":[]},
				"project":{"key":"PROJ"},"issuetype":{"name":"Story"},
				"status":{"name":"In Progress"},"assignee":{"displayName":"Jane Doe"},
				"updated":"2026-01-02T03:04:05.000+0000"}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return srv, &put
}

func assigneeConfig(base, url string) *config.Config {
	cfg := testConfig(base)
	cfg.Jira = config.JiraConfig{URL: url, Email: "e@e.com", APIToken: "t", TimeoutSeconds: 15}
	return cfg
}

func TestAssigneeShowsCurrent(t *testing.T) {
	srv, _ := assigneeServer(t, usersJaneDoe)
	defer srv.Close()
	cfg := assigneeConfig(t.TempDir(), srv.URL)

	out := captureStdout(t, func() {
		if err := Assignee(cfg, "PROJ-1", ""); err != nil {
			t.Fatalf("Assignee: %v", err)
		}
	})
	if !strings.Contains(out, "assigned to Jane Doe") {
		t.Errorf("output missing current assignee:\n%s", out)
	}
}

func TestAssigneeByDisplayName(t *testing.T) {
	srv, put := assigneeServer(t, usersJaneDoe)
	defer srv.Close()
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	cfg := assigneeConfig(base, srv.URL)

	out := captureStdout(t, func() {
		if err := Assignee(cfg, "PROJ-1", "Jane Doe"); err != nil {
			t.Fatalf("Assignee: %v", err)
		}
	})
	if !strings.Contains(out, "Assigned PROJ-1 to Jane Doe") {
		t.Errorf("output missing confirmation:\n%s", out)
	}
	if len(*put) != 1 {
		t.Fatalf("expected 1 PUT, got %d", len(*put))
	}
	fields := (*put)[0]["fields"].(map[string]any)
	assignee := fields["assignee"].(map[string]any)
	if assignee["accountId"] != "5b10a2844c20165700ede21g" {
		t.Errorf("accountId = %v", assignee["accountId"])
	}

	// cache was refreshed to the new assignee
	ticket, err := storage.NewStorage(base).ReadTicket("PROJ", "1")
	if err != nil {
		t.Fatalf("ReadTicket: %v", err)
	}
	if ticket.Assignee != "Jane Doe" {
		t.Errorf("cached assignee = %q, want Jane Doe", ticket.Assignee)
	}
}

func TestAssigneeByEmail(t *testing.T) {
	srv, put := assigneeServer(t, usersJaneDoe)
	defer srv.Close()
	cfg := assigneeConfig(t.TempDir(), srv.URL)

	if err := Assignee(cfg, "PROJ-1", "jane@corp.com"); err != nil {
		t.Fatalf("Assignee: %v", err)
	}
	if len(*put) != 1 {
		t.Fatalf("expected 1 PUT, got %d", len(*put))
	}
}

func TestAssigneeByAccountID(t *testing.T) {
	var searched bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/user/search"):
			searched = true
			t.Errorf("unexpected user search")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, `[]`)
		case strings.HasSuffix(r.URL.Path, "/issue/PROJ-1") && r.Method == http.MethodPut:
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/issue/PROJ-1"):
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":"100","key":"PROJ-1","fields":{"status":{"name":"Open"},"project":{"key":"PROJ"},"issuetype":{"name":"Story"}}}`)
		}
	}))
	defer srv.Close()
	cfg := assigneeConfig(t.TempDir(), srv.URL)

	out := captureStdout(t, func() {
		if err := Assignee(cfg, "PROJ-1", "5b10a2844c20165700ede21g"); err != nil {
			t.Fatalf("Assignee: %v", err)
		}
	})
	if searched {
		t.Fatal("account ID should not trigger a user search")
	}
	if !strings.Contains(out, "Assigned PROJ-1 to 5b10a2844c20165700ede21g") {
		t.Errorf("output missing confirmation:\n%s", out)
	}
}

func TestAssigneeExactMatchBeatsFuzzy(t *testing.T) {
	srv, put := assigneeServer(t, usersAmbiguous)
	defer srv.Close()
	cfg := assigneeConfig(t.TempDir(), srv.URL)

	if err := Assignee(cfg, "PROJ-1", "jane@acme.com"); err != nil {
		t.Fatalf("Assignee: %v", err)
	}
	if len(*put) != 1 {
		t.Fatalf("expected 1 PUT, got %d", len(*put))
	}
	fields := (*put)[0]["fields"].(map[string]any)
	assignee := fields["assignee"].(map[string]any)
	if assignee["accountId"] != "5b10a2844c20165700ede22h" {
		t.Errorf("accountId = %v, want jane@acme.com match", assignee["accountId"])
	}
}

func TestAssigneeAmbiguous(t *testing.T) {
	srv, put := assigneeServer(t, usersAmbiguous)
	defer srv.Close()
	cfg := assigneeConfig(t.TempDir(), srv.URL)

	err := Assignee(cfg, "PROJ-1", "Jane")
	if err == nil {
		t.Fatal("expected error for ambiguous user")
	}
	for _, want := range []string{
		"ambiguous",
		"1. Jane Doe <jane@corp.com> (5b10a2844c20165700ede21g)",
		"2. Jane Roe <jane@acme.com> (5b10a2844c20165700ede22h)",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}
	if len(*put) != 0 {
		t.Errorf("no PUT should happen on ambiguity, got %d", len(*put))
	}
}

func TestAssigneeAmbiguousCapsListing(t *testing.T) {
	var many strings.Builder
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&many, `{"accountId":"acct-%d","displayName":"User %d","emailAddress":"u%d@corp.com","active":true},`, i, i, i)
	}
	body := "[" + strings.TrimSuffix(many.String(), ",") + "]"

	srv, put := assigneeServer(t, body)
	defer srv.Close()
	cfg := assigneeConfig(t.TempDir(), srv.URL)

	err := Assignee(cfg, "PROJ-1", "User")
	if err == nil {
		t.Fatal("expected error for ambiguous user")
	}
	for _, want := range []string{"1. User 1", "2. User 2", "3. User 3", "... and 2 more match \"User\""} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q:\n%v", want, err)
		}
	}
	if strings.Contains(err.Error(), "4. User 4") {
		t.Errorf("listing should be capped at 3:\n%v", err)
	}
	if len(*put) != 0 {
		t.Errorf("no PUT should happen on ambiguity, got %d", len(*put))
	}
}

func TestAssigneeNoMatch(t *testing.T) {
	srv, put := assigneeServer(t, `[]`)
	defer srv.Close()
	cfg := assigneeConfig(t.TempDir(), srv.URL)

	err := Assignee(cfg, "PROJ-1", "Nobody Here")
	if err == nil {
		t.Fatal("expected error for no match")
	}
	if !strings.Contains(err.Error(), "no Jira user matches") {
		t.Errorf("error = %v", err)
	}
	if len(*put) != 0 {
		t.Errorf("no PUT should happen, got %d", len(*put))
	}
}

func TestAssigneeUnassign(t *testing.T) {
	srv, put := assigneeServer(t, usersJaneDoe)
	defer srv.Close()
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	cfg := assigneeConfig(base, srv.URL)

	out := captureStdout(t, func() {
		if err := Assignee(cfg, "PROJ-1", "unassigned"); err != nil {
			t.Fatalf("Assignee: %v", err)
		}
	})
	if !strings.Contains(out, "Unassigned PROJ-1") {
		t.Errorf("output missing confirmation:\n%s", out)
	}
	if len(*put) != 1 {
		t.Fatalf("expected 1 PUT, got %d", len(*put))
	}
	fields := (*put)[0]["fields"].(map[string]any)
	if assignee, ok := fields["assignee"]; !ok || assignee != nil {
		t.Errorf("assignee = %v, want null", assignee)
	}

	// cache was refreshed
	ticket, err := storage.NewStorage(base).ReadTicket("PROJ", "1")
	if err != nil {
		t.Fatalf("ReadTicket: %v", err)
	}
	if ticket.Assignee != "Jane Doe" {
		t.Errorf("cached assignee = %q, want Jane Doe", ticket.Assignee)
	}
}
