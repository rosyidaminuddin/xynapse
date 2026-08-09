package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func (c *JiraClient) FetchTicket(project string, ticketNum string) (*models.Ticket, error) {
	issueKey := fmt.Sprintf("%s-%s", project, ticketNum)
	endpoint := fmt.Sprintf("%s/rest/api/3/issue/%s", c.baseURL, issueKey)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error fetching issue %s: %w", issueKey, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira api error (%d): %s", resp.StatusCode, string(body))
	}

	var rawIssue models.JiraRawIssue
	if err := json.NewDecoder(resp.Body).Decode(&rawIssue); err != nil {
		return nil, fmt.Errorf("failed to parse issue JSON: %w", err)
	}

	ticket := models.MapRawToTicket(&rawIssue)
	return ticket, nil
}

// FetchSprintTickets fetches all issues in the current active sprint assigned to the
// authenticated user. Uses JQL so only open sprints for the user's projects are queried.
func (c *JiraClient) FetchSprintTickets(project string) ([]*models.Ticket, error) {
	jql := fmt.Sprintf("project = %q AND sprint in openSprints() AND assignee = currentUser()", project)

	var allIssues []models.JiraRawIssue
	startAt := 0
	for {
		params := url.Values{}
		params.Set("jql", jql)
		params.Set("startAt", fmt.Sprintf("%d", startAt))
		params.Set("maxResults", "50")
		endpoint := fmt.Sprintf("%s/rest/api/3/search?%s", c.baseURL, params.Encode())

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		c.applyHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("network error fetching sprint issues: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("jira api error (%d): %s", resp.StatusCode, string(body))
		}

		var result struct {
			Issues      []models.JiraRawIssue `json:"issues"`
			Total       int                    `json:"total"`
			MaxResults  int                    `json:"maxResults"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to parse search JSON: %w", err)
		}
		resp.Body.Close()

		allIssues = append(allIssues, result.Issues...)
		if len(allIssues) >= result.Total {
			break
		}
		startAt += len(result.Issues)
	}

	tickets := make([]*models.Ticket, 0, len(allIssues))
	for i := range allIssues {
		tickets = append(tickets, models.MapRawToTicket(&allIssues[i]))
	}
	return tickets, nil
}
