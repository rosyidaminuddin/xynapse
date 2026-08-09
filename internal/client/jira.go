package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"xynapse/internal/models"
)

type JiraClient struct {
	baseURL    string
	email      string
	apiToken   string
	httpClient *http.Client
	verbose    bool
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

// SetVerbose toggles step-by-step logging to stderr.
func (c *JiraClient) SetVerbose(verbose bool) {
	c.verbose = verbose
}

func (c *JiraClient) logf(format string, args ...any) {
	if !c.verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "[jira] "+format+"\n", args...)
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
	c.logf("GET %s", req.URL.String())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	c.logf("response status %d", resp.StatusCode)

	if c.verbose {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
		c.logf("response body: %s", string(body))
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

	c.logf("decoding issue %s response", issueKey)
	var rawIssue models.JiraRawIssue
	if err := json.NewDecoder(resp.Body).Decode(&rawIssue); err != nil {
		return nil, fmt.Errorf("failed to parse issue JSON: %w", err)
	}

	c.logf("mapping issue %s to ticket model", issueKey)
	ticket := models.MapRawToTicket(&rawIssue)
	return ticket, nil
}

// FetchActiveSprint returns the current active sprint for a board via the Agile API.
func (c *JiraClient) FetchActiveSprint(boardID string) (*models.Sprint, error) {
	endpoint := fmt.Sprintf("%s/rest/agile/1.0/board/%s/sprint?state=active", c.baseURL, boardID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(req)

	c.logf("fetching active sprint for board %s", boardID)
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

// FetchSprintTickets fetches all issues in the current active sprint assigned to the
// authenticated user, optionally filtered by issue type(s). Uses JQL so only open
// sprints for the user's projects are queried.
func (c *JiraClient) FetchSprintTickets(project string, sprintID int, types []string) ([]*models.Ticket, error) {
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

	var allIssues []models.JiraRawIssue
	startAt := 0
	for {
		params := url.Values{}
		params.Set("jql", jql)
		params.Set("startAt", fmt.Sprintf("%d", startAt))
		params.Set("maxResults", "50")
		params.Set("fields", "summary, description, project, status, assignee, updated, id, key, issuetype")
		endpoint := fmt.Sprintf("%s/rest/api/3/search?%s", c.baseURL, params.Encode())

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		c.applyHeaders(req)

		c.logf("search startAt=%d jql=%s", startAt, jql)
		resp, err := c.do(req)
		if err != nil {
			return nil, fmt.Errorf("network error fetching sprint issues: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("jira api error (%d): %s", resp.StatusCode, string(body))
		}

		var result struct {
			Issues     []models.JiraRawIssue `json:"issues"`
			Total      int                   `json:"total"`
			MaxResults int                   `json:"maxResults"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to parse search JSON: %w", err)
		}
		resp.Body.Close()

		allIssues = append(allIssues, result.Issues...)
		c.logf("page returned %d issues (total %d)", len(result.Issues), result.Total)
		if len(allIssues) >= result.Total {
			break
		}
		startAt += len(result.Issues)
	}

	c.logf("mapping %d issue(s) to ticket models", len(allIssues))
	tickets := make([]*models.Ticket, 0, len(allIssues))
	for i := range allIssues {
		tickets = append(tickets, models.MapRawToTicket(&allIssues[i]))
	}
	return tickets, nil
}
