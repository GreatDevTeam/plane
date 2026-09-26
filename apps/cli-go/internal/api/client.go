package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// DeleteComment removes an existing comment from a work item.
func (c *Client) DeleteComment(ctx context.Context, workspaceSlug, projectID, workItemID, commentID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/%s/comments/%s/", workspaceSlug, projectID, workItemID, commentID)
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}

// ListActivities returns a work item's activity/history log (field changes and its creation),
// oldest first. Comments are a separate endpoint/model and never appear in this list.
func (c *Client) ListActivities(ctx context.Context, workspaceSlug, projectID, workItemID string) ([]Activity, error) {
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/%s/activities/", workspaceSlug, projectID, workItemID)
	return listAllQuery[Activity](ctx, c, path, url.Values{"order_by": {"created_at"}})
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

// ListAttachments returns every attachment uploaded to a work item.
func (c *Client) ListAttachments(ctx context.Context, workspaceSlug, projectID, workItemID string) ([]Attachment, error) {
	var out []Attachment
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/%s/attachments/", workspaceSlug, projectID, workItemID)
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// attachmentUploadData is the presigned S3 POST CreateAttachment hands back: UploadAttachmentFile
// posts the file straight to url, with fields sent as form fields alongside it.
type attachmentUploadData struct {
	URL    string            `json:"url"`
	Fields map[string]string `json:"fields"`
}

type createAttachmentResponse struct {
	UploadData attachmentUploadData `json:"upload_data"`
	AssetID    string               `json:"asset_id"`
}

// CreateAttachment is the first of three steps to attach a local file to a work item: it
// registers the attachment and returns a presigned S3 upload (url + form fields) plus the
// asset ID that identifies it from here on. It does not transfer the file itself — follow with
// UploadAttachmentFile, then ConfirmAttachmentUploaded.
func (c *Client) CreateAttachment(ctx context.Context, workspaceSlug, projectID, workItemID, name, mimeType string, size int64) (assetID, uploadURL string, uploadFields map[string]string, err error) {
	var resp createAttachmentResponse
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/%s/attachments/", workspaceSlug, projectID, workItemID)
	body := map[string]any{"name": name, "type": mimeType, "size": size}
	if err := c.do(ctx, http.MethodPost, path, nil, body, &resp); err != nil {
		return "", "", nil, err
	}
	return resp.AssetID, resp.UploadData.URL, resp.UploadData.Fields, nil
}

// UploadAttachmentFile is the second step (see CreateAttachment): it posts a local file's
// bytes to the presigned S3 URL/fields CreateAttachment returned. This bypasses Client.do — it
// is a different host, authenticated by the presigned fields rather than the API token, and
// multipart/form-data rather than JSON.
func (c *Client) UploadAttachmentFile(ctx context.Context, uploadURL string, fields map[string]string, filePath, mimeType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return err
		}
	}
	part, err := w.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	_ = mimeType // the presigned policy's Content-Type field (in fields) governs, not this header
	if err := w.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("upload to storage failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload to storage: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

// ConfirmAttachmentUploaded is the third and final step (see CreateAttachment): without it the
// attachment stays invisible to ListAttachments (its is_uploaded flag stays false).
func (c *Client) ConfirmAttachmentUploaded(ctx context.Context, workspaceSlug, projectID, workItemID, assetID string) error {
	path := fmt.Sprintf("/api/v1/workspaces/%s/projects/%s/work-items/%s/attachments/%s/", workspaceSlug, projectID, workItemID, assetID)
	return c.do(ctx, http.MethodPatch, path, nil, map[string]any{"is_uploaded": true}, nil)
}

// DownloadAttachment fetches an attachment's file content. The attachment endpoint itself
// responds with a redirect to a presigned download URL rather than the bytes, so this follows
// that redirect manually instead of relying on Client.HTTP's default redirect policy: the
// presigned URL is a different host, and forwarding the API token onto it is both unnecessary
// and, depending on how the storage backend treats unexpected headers, can invalidate the
// signature.
func (c *Client) DownloadAttachment(ctx context.Context, workspaceSlug, projectID, workItemID, assetID string) ([]byte, error) {
	path := fmt.Sprintf("%s/api/v1/workspaces/%s/projects/%s/work-items/%s/attachments/%s/", c.BaseURL, workspaceSlug, projectID, workItemID, assetID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.Token)

	noRedirect := *c.HTTP
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noRedirect.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GET %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
		}
		return body, nil
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		return nil, fmt.Errorf("download attachment: redirect with no Location header")
	}
	downloadReq, err := http.NewRequestWithContext(ctx, http.MethodGet, loc, nil)
	if err != nil {
		return nil, err
	}
	downloadResp, err := c.HTTP.Do(downloadReq)
	if err != nil {
		return nil, fmt.Errorf("download attachment: %w", err)
	}
	defer downloadResp.Body.Close()
	if downloadResp.StatusCode < 200 || downloadResp.StatusCode >= 300 {
		return nil, fmt.Errorf("download attachment: %s", downloadResp.Status)
	}
	return io.ReadAll(downloadResp.Body)
}

// listAllPerPage is the page size every listAllQuery request asks for. It has to stay fixed
// for one listing: the parallel fetch below constructs every page's cursor itself (rather
// than chaining each page's own next_cursor), and that only lands on the right rows if every
// request in the batch agrees on the page size.
const listAllPerPage = 100

// listAllMaxParallelPages bounds how many pages listAllQuery has in flight at once, so paging
// through a very large project does not open dozens of simultaneous requests.
const listAllMaxParallelPages = 8

// listAll follows Plane's cursor pagination until every page has been fetched.
func listAll[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	return listAllQuery[T](ctx, c, path, nil)
}

// listAllQuery is listAll with extra fixed query parameters (e.g. order_by) merged into every
// page request.
//
// The first page is fetched alone — there is nothing to parallelize until it reports how many
// more pages exist — and every remaining page is then requested concurrently, rather than one
// round trip at a time as this used to. That is safe because Plane's cursor pagination is a
// plain offset/limit paginator: a cursor is "<anything>:<page number>:0", and the offset it
// decodes to is cursor.offset*limit using the request's own per_page, not anything encoded in
// the cursor's own first field (apps/api/plane/utils/paginator.py OffsetPaginator.get_result,
// "offset = cursor.offset * limit # use limit instead of cursor.value for consistent
// pagination"). So every page's cursor can be built up front from its page number instead of
// waiting for the previous page's response to hand back the next one. This is what made the
// CLI's 30s board refresh (refreshBoard -> ListWorkItems) so much slower than the equivalent
// web app fetch on a project with more than one page of work items: it walked every page
// sequentially where nothing requires that.
//
// If the server ever does not report total_count for a listing with more pages (a paginator
// this client has not seen do that), it falls back to the old page-at-a-time walk from the
// first page's next_cursor rather than silently dropping whatever the count did not cover.
func listAllQuery[T any](ctx context.Context, c *Client, path string, extra url.Values) ([]T, error) {
	q := url.Values{"per_page": {fmt.Sprint(listAllPerPage)}}
	for k, v := range extra {
		q[k] = v
	}
	var first paginatedResponse[T]
	if err := c.do(ctx, http.MethodGet, path, q, nil, &first); err != nil {
		return nil, err
	}
	if !first.NextPageResults || first.NextCursor == "" {
		return first.Results, nil
	}

	totalPages := (first.TotalCount + listAllPerPage - 1) / listAllPerPage
	if totalPages < 2 {
		// total_count did not actually cover a second page: fall back to walking the cursor
		// chain the server did give us, rather than trusting a count that disagrees with it.
		rest, err := listRemainingPagesSequential[T](ctx, c, path, extra, first.NextCursor)
		if err != nil {
			return nil, err
		}
		return append(first.Results, rest...), nil
	}

	pages := make([][]T, totalPages)
	pages[0] = first.Results

	sem := make(chan struct{}, listAllMaxParallelPages)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for p := 1; p < totalPages; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			pq := url.Values{
				"per_page": {fmt.Sprint(listAllPerPage)},
				"cursor":   {fmt.Sprintf("%d:%d:0", listAllPerPage, p)},
			}
			for k, v := range extra {
				pq[k] = v
			}
			var page paginatedResponse[T]
			if err := c.do(ctx, http.MethodGet, path, pq, nil, &page); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			pages[p] = page.Results
		}(p)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	var all []T
	for _, pg := range pages {
		all = append(all, pg...)
	}
	return all, nil
}

// listRemainingPagesSequential is listAllQuery's original page-at-a-time walk, kept as the
// fallback for a listing whose total_count does not actually cover the pages its
// next_page_results promises (see listAllQuery).
func listRemainingPagesSequential[T any](ctx context.Context, c *Client, path string, extra url.Values, cursor string) ([]T, error) {
	var all []T
	for cursor != "" {
		q := url.Values{"per_page": {fmt.Sprint(listAllPerPage)}, "cursor": {cursor}}
		for k, v := range extra {
			q[k] = v
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
