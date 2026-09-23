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

// ListWorkItemsPage fetches a single page of work items (100 per page), starting at cursor
// (pass "" for the first page) and ordered by orderBy (pass "" for the API's default). Callers
// use this instead of ListWorkItems to render a board as pages arrive rather than blocking on
// the whole project up front. Every page of one pagination sequence must use the same orderBy:
// the cursor encodes a position in that ordering.
func (c *Client) ListWorkItemsPage(ctx context.Context, workspaceSlug, projectID, cursor, orderBy string) (items []WorkItem, nextCursor string, hasNext bool, err error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/", workspaceSlug, projectID)
	q := url.Values{"per_page": {"100"}}
	if orderBy != "" {
		q.Set("order_by", orderBy)
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	var page paginatedResponse[WorkItem]
	if err := c.do(ctx, http.MethodGet, path, q, nil, &page); err != nil {
		return nil, "", false, err
	}
	return page.Results, page.NextCursor, page.NextPageResults && page.NextCursor != "", nil
}

// GetWorkItem fetches a single work item by ID. The board keeps a cached copy of every
// item, so this is what refreshes the one the user actually opened.
func (c *Client) GetWorkItem(ctx context.Context, workspaceSlug, projectID, workItemID string) (*WorkItem, error) {
	var wi WorkItem
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/%s/", workspaceSlug, projectID, workItemID)
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &wi); err != nil {
		return nil, err
	}
	return &wi, nil
}

// ListComments returns every comment on a work item, oldest first.
func (c *Client) ListComments(ctx context.Context, workspaceSlug, projectID, workItemID string) ([]Comment, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/%s/comments/", workspaceSlug, projectID, workItemID)
	return listAllQuery[Comment](ctx, c, path, url.Values{"order_by": {"created_at"}})
}

// CreateComment adds a new comment (as HTML) to a work item.
func (c *Client) CreateComment(ctx context.Context, workspaceSlug, projectID, workItemID, commentHTML string) (*Comment, error) {
	var cm Comment
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/%s/comments/", workspaceSlug, projectID, workItemID)
	if err := c.do(ctx, http.MethodPost, path, nil, map[string]any{"comment_html": commentHTML}, &cm); err != nil {
		return nil, err
	}
	return &cm, nil
}

// UpdateComment edits an existing comment's HTML body.
func (c *Client) UpdateComment(ctx context.Context, workspaceSlug, projectID, workItemID, commentID, commentHTML string) (*Comment, error) {
	var cm Comment
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/%s/comments/%s/", workspaceSlug, projectID, workItemID, commentID)
	if err := c.do(ctx, http.MethodPatch, path, nil, map[string]any{"comment_html": commentHTML}, &cm); err != nil {
		return nil, err
	}
	return &cm, nil
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

// CreateWorkItem creates a new work item with the given fields (minimum required: "name").
func (c *Client) CreateWorkItem(ctx context.Context, workspaceSlug, projectID string, fields map[string]any) (*WorkItem, error) {
	var wi WorkItem
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/", workspaceSlug, projectID)
	if err := c.do(ctx, http.MethodPost, path, nil, fields, &wi); err != nil {
		return nil, err
	}
	return &wi, nil
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
	return listAllQuery[T](ctx, c, path, nil)
}

// listAllQuery is listAll with extra fixed query parameters (e.g. order_by) merged into
// every page request.
func listAllQuery[T any](ctx context.Context, c *Client, path string, extra url.Values) ([]T, error) {
	var all []T
	cursor := ""
	for {
		q := url.Values{"per_page": {"100"}}
		for k, v := range extra {
			q[k] = v
		}
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
