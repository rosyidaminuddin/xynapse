package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"xynapse/internal/models"
)

type JiraClient struct {
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
}

func NewJiraClient(baseURL, email, apiToken string, timeout int) *JiraClient {
	return &JiraClient{
		baseURL:  baseURL,
		email:    email,
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

// Helper to construct basic auth header
func (c *JiraClient) applyHeaders(req *http.Request) {
	auth := fmt.Sprintf("%s:%s", c.email, c.apiToken)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", encodedAuth))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

func (c *JiraClient) do(req *http.Request) (*http.Response, error) {
	slog.Debug("jira request", "method", req.Method, "url", req.URL.String())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	slog.Debug("jira response", "status", resp.StatusCode)

	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
		slog.Debug("jira response body", "body", string(body))
	}

	return resp, nil
}

func (c *JiraClient) FetchTicket(project string, ticketNum string) (*models.Ticket, error) {
	issueKey := fmt.Sprintf("%s-%s", project, ticketNum)
	endpoint := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, issueKey)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(req)

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("network error fetching issue %s: %w", issueKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira api error (%d): %s", resp.StatusCode, string(body))
	}

	slog.Debug("decoding issue", "key", issueKey)
	var rawIssue models.JiraRawIssue
	if err := json.NewDecoder(resp.Body).Decode(&rawIssue); err != nil {
		return nil, fmt.Errorf("failed to parse issue JSON: %w", err)
	}

	ticket := models.MapRawToTicket(&rawIssue)
	return ticket, nil
}

// Transition describes a Jira workflow transition available for an issue.
type Transition struct {
	ID   string
	Name string
	To   string // the status the issue lands in after the transition
}

// FetchTransitions returns the workflow transitions available for an issue,
// each with its target status.
func (c *JiraClient) FetchTransitions(project, ticketNum string) ([]Transition, error) {
	issueKey := fmt.Sprintf("%s-%s", project, ticketNum)
	endpoint := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, issueKey)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(req)

	slog.Debug("fetching transitions", "issue", issueKey)
	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("network error fetching transitions for %s: %w", issueKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira api error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name string `json:"name"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse transitions JSON: %w", err)
	}

	transitions := make([]Transition, 0, len(result.Transitions))
	for _, t := range result.Transitions {
		transitions = append(transitions, Transition{
			ID:   t.ID,
			Name: t.Name,
			To:   t.To.Name,
		})
	}
	return transitions, nil
}

// TransitionTicket moves an issue to a new status via the given transition ID.
func (c *JiraClient) TransitionTicket(project, ticketNum, transitionID string) error {
	issueKey := fmt.Sprintf("%s-%s", project, ticketNum)
	endpoint := fmt.Sprintf("%s/rest/api/3/issue/%s/transitions", c.baseURL, issueKey)

	body := struct {
		Transition struct {
			ID string `json:"id"`
		} `json:"transition"`
	}{}
	body.Transition.ID = transitionID

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode transition request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(req)

	slog.Debug("transitioning issue", "issue", issueKey, "transition_id", transitionID)
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("network error transitioning %s: %w", issueKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira api error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// AddComment posts a plain-text comment to an issue. The body is converted to
// the Atlassian Document Format (one paragraph per line) so Jira renders it
// as a normal comment.
func (c *JiraClient) AddComment(project, ticketNum, body string) error {
	issueKey := fmt.Sprintf("%s-%s", project, ticketNum)
	endpoint := fmt.Sprintf("%s/rest/api/3/issue/%s/comment", c.baseURL, issueKey)

	payload, err := json.Marshal(struct {
		Body any `json:"body"`
	}{Body: commentADF(body)})
	if err != nil {
		return fmt.Errorf("failed to encode comment request: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(req)

	slog.Debug("adding comment to issue", "issue", issueKey)
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("network error adding comment to %s: %w", issueKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira api error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// commentADF builds an ADF document whose paragraphs carry the lines of text,
// skipping empty lines. This keeps the payload small while preserving
// line breaks in the rendered comment.
func commentADF(text string) map[string]any {
	paragraphs := make([]any, 0)
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		paragraphs = append(paragraphs, map[string]any{
			"type": "paragraph",
			"content": []any{map[string]any{
				"type": "text",
				"text": line,
			}},
		})
	}
	return map[string]any{
		"version": 1,
		"type":    "doc",
		"content": paragraphs,
	}
}

// JiraUser is a Jira account returned by the user search endpoint.
type JiraUser struct {
	AccountID   string
	DisplayName string
	Email       string
}

// SearchUsers looks up Jira users by display name or email address via the
// user search endpoint.
func (c *JiraClient) SearchUsers(query string) ([]JiraUser, error) {
	params := url.Values{}
	params.Set("query", query)
	params.Set("maxResults", "20")
	endpoint := fmt.Sprintf("%s/rest/api/3/user/search?%s", c.baseURL, params.Encode())

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(req)

	slog.Debug("searching jira users", "query", query)
	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("network error searching users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira api error (%d): %s", resp.StatusCode, string(body))
	}

	var raw []struct {
		AccountID   string `json:"accountId"`
		DisplayName string `json:"displayName"`
		Email       string `json:"emailAddress"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to parse user search JSON: %w", err)
	}

	users := make([]JiraUser, 0, len(raw))
	for _, u := range raw {
		users = append(users, JiraUser{
			AccountID:   u.AccountID,
			DisplayName: u.DisplayName,
			Email:       u.Email,
		})
	}
	return users, nil
}

// SetAssignee assigns a ticket to the given account, or unassigns it when
// unassign is true. Jira returns 204 No Content on success.
func (c *JiraClient) SetAssignee(project, ticketNum, accountID string, unassign bool) error {
	issueKey := fmt.Sprintf("%s-%s", project, ticketNum)
	endpoint := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, issueKey)

	fields := struct {
		Assignee any `json:"assignee"`
	}{}
	if unassign {
		fields.Assignee = nil
	} else {
		fields.Assignee = struct {
			AccountID string `json:"accountId"`
		}{AccountID: accountID}
	}

	payload, err := json.Marshal(struct {
		Fields any `json:"fields"`
	}{Fields: fields})
	if err != nil {
		return fmt.Errorf("failed to encode assignee request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(req)

	slog.Debug("updating assignee", "issue", issueKey, "unassign", unassign)
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("network error updating assignee for %s: %w", issueKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira api error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// FetchActiveSprint returns the current active sprint for a board via the Agile API.
func (c *JiraClient) FetchActiveSprint(boardID string) (*models.Sprint, error) {
	endpoint := fmt.Sprintf("%s/rest/agile/1.0/board/%s/sprint?state=active", c.baseURL, boardID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(req)

	slog.Debug("fetching active sprint", "board_id", boardID)
	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("network error fetching active sprint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira api error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Values []models.Sprint `json:"values"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse sprint JSON: %w", err)
	}

	for i := range result.Values {
		if result.Values[i].State == "active" {
			return &result.Values[i], nil
		}
	}
	return nil, fmt.Errorf("no active sprint found for board %s", boardID)
}

// BuildSprintJQL constructs the JQL query for the current active sprint of the
// authenticated user, optionally scoped to the given sprint and issue types.
func BuildSprintJQL(project string, sprintID int, types []string) string {
	jql := fmt.Sprintf("project = %q AND sprint in openSprints() AND assignee = currentUser()", project)
	if sprintID > 0 {
		jql = fmt.Sprintf("project = %q AND sprint = %d AND assignee = currentUser()", project, sprintID)
	}
	if len(types) > 0 {
		quoted := make([]string, len(types))
		for i, t := range types {
			quoted[i] = fmt.Sprintf("%q", t)
		}
		jql += fmt.Sprintf(" AND issuetype in (%s)", strings.Join(quoted, ","))
	}
	return jql
}

// SearchIssues runs a JQL query against the enhanced search endpoint
// (/rest/api/3/search/jql), following nextPageToken pagination.
func (c *JiraClient) SearchIssues(jql string) ([]models.JiraRawIssue, error) {
	var allIssues []models.JiraRawIssue
	nextPageToken := ""

	for {
		params := url.Values{}
		params.Set("jql", jql)
		params.Set("maxResults", "50")
		params.Set("fields", "summary, description, project, status, assignee, updated, id, key, issuetype")
		if nextPageToken != "" {
			params.Set("nextPageToken", nextPageToken)
		}
		endpoint := fmt.Sprintf("%s/rest/api/3/search/jql?%s", c.baseURL, params.Encode())

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		c.applyHeaders(req)

		slog.Debug("jira search", "jql", jql, "nextPageToken", nextPageToken)
		resp, err := c.do(req)
		if err != nil {
			return nil, fmt.Errorf("network error searching issues: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("jira api error (%d): %s", resp.StatusCode, string(body))
		}

		var result struct {
			Issues        []models.JiraRawIssue `json:"issues"`
			NextPageToken string                `json:"nextPageToken"`
			IsLast        bool                  `json:"isLast"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to parse search JSON: %w", err)
		}
		resp.Body.Close()

		allIssues = append(allIssues, result.Issues...)
		slog.Debug("jira search page", "issues", len(result.Issues), "is_last", result.IsLast)
		if result.IsLast {
			break
		}
		nextPageToken = result.NextPageToken
	}

	return allIssues, nil
}

// FetchSprintTickets fetches all issues in the current active sprint assigned to the
// authenticated user, optionally filtered by issue type(s).
func (c *JiraClient) FetchSprintTickets(project string, sprintID int, types []string) ([]*models.Ticket, error) {
	jql := BuildSprintJQL(project, sprintID, types)

	issues, err := c.SearchIssues(jql)
	if err != nil {
		return nil, err
	}

	tickets := make([]*models.Ticket, 0, len(issues))
	for i := range issues {
		tickets = append(tickets, models.MapRawToTicket(&issues[i]))
	}
	return tickets, nil
}
