package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to a single Plane server's public REST API using an API token.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New creates a client for the given server base URL (e.g. "https://app.plane.so" or
// "http://localhost:8000") and API token.
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(b)
	}

	full := c.BaseURL + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, full, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		// net/http wraps transport failures in a *url.Error whose Error() repeats the
		// full request URL (including query string); unwrap it so a connectivity error
		// reads as "GET /path failed: <cause>" instead of dumping the whole URL.
		cause := err
		if uerr, ok := err.(*url.Error); ok {
			cause = uerr.Err
		}
		return fmt.Errorf("%s %s failed: %w", method, path, cause)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiError
		_ = json.Unmarshal(respBody, &apiErr)
		msg := apiErr.Error
		if msg == "" {
			msg = apiErr.Detail
		}
		if msg == "" {
			msg = strings.TrimSpace(string(respBody))
		}
		return fmt.Errorf("%s %s: %s (%s)", method, path, resp.Status, msg)
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

// Me returns the authenticated user, and doubles as a token-validity check.
func (c *Client) Me(ctx context.Context) (*User, error) {
	var u User
	if err := c.do(ctx, http.MethodGet, "/api/v1/users/me/", nil, nil, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// ListProjects returns every project in the given workspace.
func (c *Client) ListProjects(ctx context.Context, workspaceSlug string) ([]Project, error) {
	return listAll[Project](ctx, c, fmt.Sprintf("/api/v1/workspaces/%s/projects/", workspaceSlug))
}

// ListStates returns every work item state for the given project, ordered by sequence.
func (c *Client) ListStates(ctx context.Context, workspaceSlug, projectID string) ([]State, error) {
	states, err := listAll[State](ctx, c, fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/states/", workspaceSlug, projectID))
	if err != nil {
		return nil, err
	}
	sortStates(states)
	return states, nil
}

// ListWorkItems returns every work item in the given project.
func (c *Client) ListWorkItems(ctx context.Context, workspaceSlug, projectID string) ([]WorkItem, error) {
	return listAll[WorkItem](ctx, c, fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/", workspaceSlug, projectID))
}

// ListLabels returns every label defined on the given project.
func (c *Client) ListLabels(ctx context.Context, workspaceSlug, projectID string) ([]Label, error) {
	return listAll[Label](ctx, c, fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/labels/", workspaceSlug, projectID))
}

// ListMembers returns every member of the given project (used to resolve assignee IDs to
// display names). Unlike the other list endpoints this one is not paginated: it returns a
// plain JSON array.
func (c *Client) ListMembers(ctx context.Context, workspaceSlug, projectID string) ([]Member, error) {
	var members []Member
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/members/", workspaceSlug, projectID)
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &members); err != nil {
		return nil, err
	}
	return members, nil
}

// UpdateWorkItem PATCHes the given fields (e.g. {"state": id} or {"priority": "high"}) on a work item.
func (c *Client) UpdateWorkItem(ctx context.Context, workspaceSlug, projectID, workItemID string, patch map[string]any) (*WorkItem, error) {
	var wi WorkItem
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/%s/", workspaceSlug, projectID, workItemID)
	if err := c.do(ctx, http.MethodPatch, path, nil, patch, &wi); err != nil {
		return nil, err
	}
	return &wi, nil
}

// listAll follows Plane's cursor pagination until every page has been fetched.
func listAll[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var all []T
	cursor := ""
	for {
		q := url.Values{"per_page": {"100"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var page paginatedResponse[T]
		if err := c.do(ctx, http.MethodGet, path, q, nil, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Results...)
		if !page.NextPageResults || page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	return all, nil
}

func sortStates(states []State) {
	for i := 1; i < len(states); i++ {
		for j := i; j > 0 && states[j].Sequence < states[j-1].Sequence; j-- {
			states[j], states[j-1] = states[j-1], states[j]
		}
	}
}
