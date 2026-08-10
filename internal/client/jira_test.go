package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildSprintJQL(t *testing.T) {
	cases := []struct {
		name     string
		project  string
		sprintID int
		types    []string
		wantSub  []string
	}{
		{
			name:     "no sprint id",
			project:  "MERADIO",
			sprintID: 0,
			types:    nil,
			wantSub:  []string{`project = "MERADIO"`, "sprint in openSprints()", "assignee = currentUser()"},
		},
		{
			name:     "with sprint id",
			project:  "MERADIO",
			sprintID: 42,
			types:    nil,
			wantSub:  []string{`project = "MERADIO"`, "sprint = 42", "assignee = currentUser()"},
		},
		{
			name:     "single type",
			project:  "MERADIO",
			sprintID: 0,
			types:    []string{"Story"},
			wantSub:  []string{`issuetype in ("Story")`},
		},
		{
			name:     "multiple types",
			project:  "MERADIO",
			sprintID: 0,
			types:    []string{"Story", "Bug", "Epic"},
			wantSub:  []string{`issuetype in ("Story","Bug","Epic")`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jql := BuildSprintJQL(tc.project, tc.sprintID, tc.types)
			for _, sub := range tc.wantSub {
				if !strings.Contains(jql, sub) {
					t.Errorf("JQL %q does not contain %q", jql, sub)
				}
			}
		})
	}
}

func TestBuildSprintJQLTypeExcludesOpenSprints(t *testing.T) {
	// With a sprint id present, the openSprints() clause must not appear.
	jql := BuildSprintJQL("MERADIO", 42, []string{"Story"})
	if strings.Contains(jql, "openSprints()") {
		t.Errorf("JQL %q should not contain openSprints() when sprint id given", jql)
	}
}

func TestFetchTransitions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rest/api/3/issue/PROJ-1/transitions") {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"expand":"transitions","transitions":[
			{"id":"11","name":"To Do","to":{"name":"To Do"}},
			{"id":"21","name":"In Progress","to":{"name":"In Progress"}}
		]}`)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, "e@e.com", "t", 15)
	got, err := c.FetchTransitions("PROJ", "1")
	if err != nil {
		t.Fatalf("FetchTransitions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d transitions, want 2", len(got))
	}
	if got[0].ID != "11" || got[0].Name != "To Do" || got[0].To != "To Do" {
		t.Errorf("transition 0 = %+v", got[0])
	}
	if got[1].ID != "21" || got[1].To != "In Progress" {
		t.Errorf("transition 1 = %+v", got[1])
	}
}

func TestFetchTransitionsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"errorMessages":["Issue Does Not Exist"]}`)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, "e@e.com", "t", 15)
	_, err := c.FetchTransitions("PROJ", "999")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err = %v, want 404", err)
	}
}

func TestTransitionTicket(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rest/api/3/issue/PROJ-1/transitions") {
			t.Errorf("path = %s", r.URL.Path)
		}
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, "e@e.com", "t", 15)
	if err := c.TransitionTicket("PROJ", "1", "21"); err != nil {
		t.Fatalf("TransitionTicket: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("invalid request body %q: %v", gotBody, err)
	}
	tr := body["transition"].(map[string]any)
	if tr["id"] != "21" {
		t.Errorf("transition.id = %v, want 21", tr["id"])
	}
}

func TestTransitionTicketError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"errorMessages":["bad transition"]}`)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, "e@e.com", "t", 15)
	err := c.TransitionTicket("PROJ", "1", "99")
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("err = %v, want 400", err)
	}
}

func TestSearchUsers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rest/api/3/user/search") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "jane" {
			t.Errorf("query param = %q, want jane", r.URL.Query().Get("query"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `[
			{"accountId":"5b10a2844c20165700ede21g","displayName":"Jane Doe","emailAddress":"jane@corp.com","active":true},
			{"accountId":"5b10a2844c20165700ede22h","displayName":"Jane Roe","emailAddress":"jane@acme.com","active":true}
		]`)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, "e@e.com", "t", 15)
	got, err := c.SearchUsers("jane")
	if err != nil {
		t.Fatalf("SearchUsers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d users, want 2", len(got))
	}
	if got[0].AccountID != "5b10a2844c20165700ede21g" || got[0].DisplayName != "Jane Doe" || got[0].Email != "jane@corp.com" {
		t.Errorf("user 0 = %+v", got[0])
	}
}

func TestSearchUsersError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"errorMessages":["You do not have permission to browse users"]}`)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, "e@e.com", "t", 15)
	_, err := c.SearchUsers("jane")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want 403", err)
	}
}

func TestSetAssignee(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rest/api/3/issue/PROJ-1") {
			t.Errorf("path = %s", r.URL.Path)
		}
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, "e@e.com", "t", 15)
	if err := c.SetAssignee("PROJ", "1", "5b10a2844c20165700ede21g", false); err != nil {
		t.Fatalf("SetAssignee: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("invalid request body %q: %v", gotBody, err)
	}
	fields := body["fields"].(map[string]any)
	assignee := fields["assignee"].(map[string]any)
	if assignee["accountId"] != "5b10a2844c20165700ede21g" {
		t.Errorf("assignee.accountId = %v", assignee["accountId"])
	}
}

func TestUnassign(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, "e@e.com", "t", 15)
	if err := c.SetAssignee("PROJ", "1", "", true); err != nil {
		t.Fatalf("SetAssignee: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("invalid request body %q: %v", gotBody, err)
	}
	fields := body["fields"].(map[string]any)
	assignee, ok := fields["assignee"]
	if !ok || assignee != nil {
		t.Errorf("assignee = %v, want null", assignee)
	}
}

func TestCommentADF(t *testing.T) {
	doc := commentADF("line one\n\nline two")
	content, ok := doc["content"].([]any)
	if !ok {
		t.Fatalf("content = %T, want []any", doc["content"])
	}
	if len(content) != 2 {
		t.Fatalf("got %d paragraphs, want 2 (empty line skipped)", len(content))
	}
	p := content[0].(map[string]any)
	if p["type"] != "paragraph" {
		t.Errorf("paragraph type = %v", p["type"])
	}
	texts := p["content"].([]any)[0].(map[string]any)
	if texts["text"] != "line one" {
		t.Errorf("paragraph text = %v", texts["text"])
	}
	if doc["type"] != "doc" || doc["version"] != 1 {
		t.Errorf("doc = %v", doc)
	}
}

func TestAddComment(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rest/api/3/issue/PROJ-1/comment") {
			t.Errorf("path = %s", r.URL.Path)
		}
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, "e@e.com", "t", 15)
	if err := c.AddComment("PROJ", "1", "PR merged"); err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("invalid request body %q: %v", gotBody, err)
	}
	doc, ok := body["body"].(map[string]any)
	if !ok {
		t.Fatalf("body = %T, want map", body["body"])
	}
	if doc["type"] != "doc" {
		t.Errorf("doc type = %v", doc["type"])
	}
}

func TestAddCommentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"errorMessages":["You do not have permission"]}`)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, "e@e.com", "t", 15)
	err := c.AddComment("PROJ", "1", "hi")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want 403", err)
	}
}

func TestSetAssigneeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"errorMessages":["The user does not exist"]}`)
	}))
	defer srv.Close()

	c := NewJiraClient(srv.URL, "e@e.com", "t", 15)
	err := c.SetAssignee("PROJ", "1", "bad-id", false)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("err = %v, want 400", err)
	}
}
