package command

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xynapse/internal/config"
	"xynapse/internal/storage"
)

const (
	transitionsUnique = `{"transitions":[
		{"id":"11","name":"To Do","to":{"name":"To Do"}},
		{"id":"21","name":"In Progress","to":{"name":"In Progress"}}
	]}`

	transitionsAmbiguous = `{"transitions":[
		{"id":"11","name":"To Do","to":{"name":"To Do"}},
		{"id":"21","name":"In Progress","to":{"name":"In Progress"}},
		{"id":"41","name":"Back In Progress","to":{"name":"In Progress"}}
	]}`
)

// transitionServer mocks the Jira API for transition tests, serving the given
// transitions payload. On POST it records the request body and returns 204;
// the subsequent ticket fetch returns status "In Progress".
func transitionServer(t *testing.T, transitionsJSON string) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var posted []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/transitions") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, transitionsJSON)
		case strings.HasSuffix(r.URL.Path, "/transitions") && r.Method == http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("bad request body: %v", err)
			}
			posted = append(posted, body)
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/issue/PROJ-1"):
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"id":"100","key":"PROJ-1","fields":{
				"summary":"Do stuff","description":{"type":"doc","version":1,"content":[]},
				"project":{"key":"PROJ"},"issuetype":{"name":"Story"},
				"status":{"name":"In Progress"},"assignee":{"displayName":"Me"},
				"updated":"2026-01-02T03:04:05.000+0000"}}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return srv, &posted
}

func transitionConfig(base, url string) *config.Config {
	cfg := testConfig(base)
	cfg.Jira = config.JiraConfig{URL: url, Email: "e@e.com", APIToken: "t", TimeoutSeconds: 15}
	return cfg
}

func TestTransitionLists(t *testing.T) {
	srv, _ := transitionServer(t, transitionsAmbiguous)
	defer srv.Close()
	cfg := transitionConfig(t.TempDir(), srv.URL)

	out := captureStdout(t, func() {
		if err := Transition(cfg, "PROJ-1", "", ""); err != nil {
			t.Fatalf("Transition: %v", err)
		}
	})
	for _, want := range []string{"11", "To Do", "In Progress", "Back In Progress"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestTransitionByStatus(t *testing.T) {
	srv, posted := transitionServer(t, transitionsUnique)
	defer srv.Close()
	base := t.TempDir()
	writeTicket(t, base, "PROJ-1")
	cfg := transitionConfig(base, srv.URL)

	out := captureStdout(t, func() {
		if err := Transition(cfg, "PROJ-1", "in progress", ""); err != nil {
			t.Fatalf("Transition: %v", err)
		}
	})
	if !strings.Contains(out, "Transitioned PROJ-1 to In Progress") {
		t.Errorf("output missing confirmation:\n%s", out)
	}
	if len(*posted) != 1 {
		t.Fatalf("expected 1 POST, got %d", len(*posted))
	}
	if (*posted)[0]["transition"].(map[string]any)["id"] != "21" {
		t.Errorf("transition id = %v, want 21", (*posted)[0]["transition"])
	}

	// cache was refreshed to the new status
	ticket, err := storage.NewStorage(base).ReadTicket("PROJ", "1")
	if err != nil {
		t.Fatalf("ReadTicket: %v", err)
	}
	if ticket.Status != "In Progress" {
		t.Errorf("cached status = %q, want In Progress", ticket.Status)
	}
}

func TestTransitionNoMatch(t *testing.T) {
	srv, _ := transitionServer(t, transitionsAmbiguous)
	defer srv.Close()
	cfg := transitionConfig(t.TempDir(), srv.URL)

	err := Transition(cfg, "PROJ-1", "Blocked", "")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no transition to") || !strings.Contains(err.Error(), "available") {
		t.Errorf("error = %v", err)
	}
}

func TestTransitionAmbiguous(t *testing.T) {
	srv, posted := transitionServer(t, transitionsAmbiguous)
	defer srv.Close()
	cfg := transitionConfig(t.TempDir(), srv.URL)

	err := Transition(cfg, "PROJ-1", "in progress", "")
	if err == nil {
		t.Fatal("expected error for ambiguous status")
	}
	if !strings.Contains(err.Error(), "multiple transitions") || !strings.Contains(err.Error(), "--id") {
		t.Errorf("error = %v", err)
	}
	if len(*posted) != 0 {
		t.Errorf("no transition should be posted on ambiguity, got %d", len(*posted))
	}
}

func TestTransitionByID(t *testing.T) {
	srv, posted := transitionServer(t, transitionsAmbiguous)
	defer srv.Close()
	cfg := transitionConfig(t.TempDir(), srv.URL)

	out := captureStdout(t, func() {
		if err := Transition(cfg, "PROJ-1", "", "41"); err != nil {
			t.Fatalf("Transition: %v", err)
		}
	})
	if !strings.Contains(out, "Transitioned PROJ-1 to In Progress") {
		t.Errorf("output missing confirmation:\n%s", out)
	}
	if len(*posted) != 1 {
		t.Fatalf("expected 1 POST, got %d", len(*posted))
	}
	if (*posted)[0]["transition"].(map[string]any)["id"] != "41" {
		t.Errorf("transition id = %v, want 41", (*posted)[0]["transition"])
	}
}

func TestTransitionUnknownID(t *testing.T) {
	srv, posted := transitionServer(t, transitionsAmbiguous)
	defer srv.Close()
	cfg := transitionConfig(t.TempDir(), srv.URL)

	err := Transition(cfg, "PROJ-1", "", "999")
	if err == nil {
		t.Fatal("expected error for unknown transition id")
	}
	if len(*posted) != 0 {
		t.Errorf("no transition should be posted, got %d", len(*posted))
	}
}

func TestTransitionByTransitionName(t *testing.T) {
	srv, posted := transitionServer(t, transitionsAmbiguous)
	defer srv.Close()
	cfg := transitionConfig(t.TempDir(), srv.URL)

	out := captureStdout(t, func() {
		if err := Transition(cfg, "PROJ-1", "Back In Progress", ""); err != nil {
			t.Fatalf("Transition: %v", err)
		}
	})
	if !strings.Contains(out, "Transitioned PROJ-1 to In Progress") {
		t.Errorf("output missing confirmation:\n%s", out)
	}
	if len(*posted) != 1 {
		t.Fatalf("expected 1 POST, got %d", len(*posted))
	}
	if (*posted)[0]["transition"].(map[string]any)["id"] != "41" {
		t.Errorf("transition id = %v, want 41", (*posted)[0]["transition"])
	}
}
