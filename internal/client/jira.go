package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
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

// Retry configuration for transient Jira failures.
const (
	maxRetries     = 3
	retryBaseDelay = time.Second
)

// retryableStatus reports whether the status warrants a retry: rate limited or
// a transient server error.
func retryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// cloneRequest returns a fresh copy of req that can be sent again, rebuilding
// its body via GetBody (available whenever the body was a bytes.Reader,
// bytes.Buffer, or strings.Reader). The returned copy is independent of the
// consumed original.
func cloneRequest(req *http.Request) (*http.Request, error) {
	body := req.Body
	if req.GetBody != nil {
		b, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		body = b
	}
	clone := req.Clone(context.Background())
	clone.Body = body
	if body != nil {
		clone.ContentLength = req.ContentLength
	}
	return clone, nil
}

// retryDelay computes the sleep before the next attempt: the Retry-After
// header when present, otherwise exponential backoff from retryBaseDelay.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	if v := resp.Header.Get("Retry-After"); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	d := retryBaseDelay << attempt
	if d < retryBaseDelay {
		d = retryBaseDelay
	}
	return d
}

func (c *JiraClient) do(req *http.Request) (*http.Response, error) {
	attempt := 0
	for {
		slog.Debug("jira request", "method", req.Method, "url", req.URL.String(), "attempt", attempt)
		sendReq := req
		var err error
		if attempt > 0 {
			sendReq, err = cloneRequest(req)
			if err != nil {
				return nil, fmt.Errorf("jira request rebuild failed: %w", err)
			}
		}
		resp, err := c.httpClient.Do(sendReq)
		if err != nil {
			return nil, err
		}
		slog.Debug("jira response", "status", resp.StatusCode, "attempt", attempt)

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

		if !retryableStatus(resp.StatusCode) || attempt >= maxRetries-1 {
			return resp, nil
		}

		// Drain and close so the connection can be reused on the next attempt.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		slog.Debug("retrying jira request", "status", resp.StatusCode, "attempt", attempt, "next_in", retryDelay(resp, attempt))
		time.Sleep(retryDelay(resp, attempt))
		attempt++
	}
}

// apiError formats a Jira API error, adding a credentials hint on 401.
func apiError(statusCode int, body string) error {
	msg := fmt.Sprintf("jira api error (%d): %s", statusCode, body)
	if statusCode == http.StatusUnauthorized {
		msg += " (check jira.email / jira.api_token in ~/.config/xynapse/config.yaml or the JIRA_EMAIL/JIRA_API_TOKEN environment variables)"
	}
	return errors.New(msg)
}

func (c *JiraClient) FetchTicket(project string, ticketNum string, acceptanceCriteriaField string) (*models.Ticket, error) {
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
		return nil, apiError(resp.StatusCode, string(body))
	}

	slog.Debug("decoding issue", "key", issueKey)
	var rawIssue models.JiraRawIssue
	if err := json.NewDecoder(resp.Body).Decode(&rawIssue); err != nil {
		return nil, fmt.Errorf("failed to parse issue JSON: %w", err)
	}

	ticket := models.MapRawToTicket(&rawIssue, acceptanceCriteriaField)
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
		return nil, apiError(resp.StatusCode, string(body))
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
		return apiError(resp.StatusCode, string(body))
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
		return apiError(resp.StatusCode, string(body))
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
		return nil, apiError(resp.StatusCode, string(body))
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
		return apiError(resp.StatusCode, string(body))
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
		return nil, apiError(resp.StatusCode, string(body))
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

// jqlString renders s as a double-quoted JQL string literal. JQL uses
// backslash escapes only for \" and \\; Go's %q is not JQL-compatible because
// it also mangles non-ASCII and other characters.
func jqlString(s string) string {
	return `"` + escapeJQL(s) + `"`
}

// escapeJQL escapes the two characters JQL treats specially inside a
// double-quoted string literal: backslash and double quote.
func escapeJQL(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`)
}

// BuildSprintJQL constructs the JQL query for the current active sprint of the
// authenticated user, optionally scoped to the given sprint and issue types.
func BuildSprintJQL(project string, sprintID int, types []string) string {
	jql := fmt.Sprintf("project = %s AND sprint in openSprints() AND assignee = currentUser()", jqlString(project))
	if sprintID > 0 {
		jql = fmt.Sprintf("project = %s AND sprint = %d AND assignee = currentUser()", jqlString(project), sprintID)
	}
	if len(types) > 0 {
		quoted := make([]string, len(types))
		for i, t := range types {
			quoted[i] = jqlString(t)
		}
		jql += fmt.Sprintf(" AND issuetype in (%s)", strings.Join(quoted, ","))
	}
	return jql
}

// defaultIssueFields are the fields requested from the search endpoint. Callers
// can add custom field ids via SearchIssues.
var defaultIssueFields = []string{
	"summary", "description", "project", "status", "assignee", "updated", "id", "key", "issuetype",
}

// SearchIssues runs a JQL query against the enhanced search endpoint
// (/rest/api/3/search/jql), following nextPageToken pagination. extraFields are
// appended to the requested fields so custom fields are returned by the API.
func (c *JiraClient) SearchIssues(jql string, extraFields []string) ([]models.JiraRawIssue, error) {
	fields := defaultIssueFields
	fields = append(fields, extraFields...)

	var allIssues []models.JiraRawIssue
	nextPageToken := ""

	for {
		params := url.Values{}
		params.Set("jql", jql)
		params.Set("maxResults", "50")
		params.Set("fields", strings.Join(fields, ","))
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
			return nil, apiError(resp.StatusCode, string(body))
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
		// A server that omits nextPageToken while claiming more pages would
		// otherwise loop forever; bail out instead.
		if result.NextPageToken == "" {
			slog.Warn("jira search ended without a nextPageToken while isLast=false; stopping pagination")
			break
		}
		nextPageToken = result.NextPageToken
	}

	return allIssues, nil
}

// FetchSprintTickets fetches all issues in the current active sprint assigned to the
// authenticated user, optionally filtered by issue type(s). acceptanceCriteriaField,
// when non-empty, is requested and stored on each ticket. sprintJQL, when non-empty,
// fully overrides the built-in sprint query (for projects with a custom sprint scheme).
func (c *JiraClient) FetchSprintTickets(project string, sprintID int, types []string, acceptanceCriteriaField string, sprintJQL string) ([]*models.Ticket, error) {
	jql := sprintJQL
	if jql == "" {
		jql = BuildSprintJQL(project, sprintID, types)
	} else if len(types) > 0 {
		quoted := make([]string, len(types))
		for i, t := range types {
			quoted[i] = jqlString(t)
		}
		jql += fmt.Sprintf(" AND issuetype in (%s)", strings.Join(quoted, ","))
	}

	var extraFields []string
	if acceptanceCriteriaField != "" {
		extraFields = []string{acceptanceCriteriaField}
	}
	issues, err := c.SearchIssues(jql, extraFields)
	if err != nil {
		return nil, err
	}

	tickets := make([]*models.Ticket, 0, len(issues))
	for i := range issues {
		tickets = append(tickets, models.MapRawToTicket(&issues[i], acceptanceCriteriaField))
	}
	return tickets, nil
}
