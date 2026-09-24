package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestListWorkItemsPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") == "" {
			_ = json.NewEncoder(w).Encode(paginatedResponse[WorkItem]{
				Results:         []WorkItem{{ID: "1"}, {ID: "2"}},
				NextCursor:      "page2",
				NextPageResults: true,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(paginatedResponse[WorkItem]{
			Results:         []WorkItem{{ID: "3"}},
			NextPageResults: false,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")

	items, cursor, hasNext, err := c.ListWorkItemsPage(context.Background(), "ws", "proj", "", "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(items) != 2 || cursor != "page2" || !hasNext {
		t.Fatalf("first page = %+v, %q, %v; want 2 items, cursor page2, hasNext true", items, cursor, hasNext)
	}

	items, cursor, hasNext, err = c.ListWorkItemsPage(context.Background(), "ws", "proj", cursor, "")
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(items) != 1 || cursor != "" || hasNext {
		t.Fatalf("second page = %+v, %q, %v; want 1 item, no next", items, cursor, hasNext)
	}
}

// TestListWorkItemsPageOrderBy checks orderBy is forwarded as order_by when set, and left off
// the query entirely when empty — the board's page requests always pass one (boardItemOrderBy)
// so its cursor pagination stays consistent, but the parameter itself is optional API-side.
func TestListWorkItemsPageOrderBy(t *testing.T) {
	var gotOrderBy string
	var sawOrderByParam bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOrderBy = r.URL.Query().Get("order_by")
		sawOrderByParam = r.URL.Query().Has("order_by")
		_ = json.NewEncoder(w).Encode(paginatedResponse[WorkItem]{Results: []WorkItem{{ID: "1"}}})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")

	if _, _, _, err := c.ListWorkItemsPage(context.Background(), "ws", "proj", "", "state__group"); err != nil {
		t.Fatalf("ListWorkItemsPage: %v", err)
	}
	if gotOrderBy != "state__group" {
		t.Errorf("order_by = %q, want state__group", gotOrderBy)
	}

	if _, _, _, err := c.ListWorkItemsPage(context.Background(), "ws", "proj", "", ""); err != nil {
		t.Fatalf("ListWorkItemsPage: %v", err)
	}
	if sawOrderByParam {
		t.Error("empty orderBy should not send an order_by query param at all")
	}
}

// TestListAllQueryParallelPages checks that a listing spanning several pages fetches the
// first page once and every later page exactly once, and reassembles them in page order even
// though they are requested concurrently — see listAllQuery.
func TestListAllQueryParallelPages(t *testing.T) {
	var firstPageRequests int32
	seen := map[int]int32{}
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		page := 0
		if cursor == "" {
			atomic.AddInt32(&firstPageRequests, 1)
		} else {
			bits := strings.Split(cursor, ":")
			page, _ = strconv.Atoi(bits[1])
		}
		mu.Lock()
		seen[page]++
		mu.Unlock()

		_ = json.NewEncoder(w).Encode(paginatedResponse[WorkItem]{
			Results:         []WorkItem{{ID: fmt.Sprintf("p%d-a", page)}, {ID: fmt.Sprintf("p%d-b", page)}},
			TotalCount:      250, // three pages at per_page=100
			NextCursor:      "100:1:0",
			NextPageResults: page == 0,
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	items, err := listAll[WorkItem](context.Background(), c, "/x/")
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}

	want := []string{"p0-a", "p0-b", "p1-a", "p1-b", "p2-a", "p2-b"}
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d: %+v", len(items), len(want), items)
	}
	for i, id := range want {
		if items[i].ID != id {
			t.Errorf("items[%d].ID = %q, want %q (pages must reassemble in page order)", i, items[i].ID, id)
		}
	}

	if firstPageRequests != 1 {
		t.Errorf("first page requested %d times, want 1", firstPageRequests)
	}
	for p := 0; p < 3; p++ {
		if seen[p] != 1 {
			t.Errorf("page %d requested %d times, want 1", p, seen[p])
		}
	}
}

func TestListComments(t *testing.T) {
	var gotOrderBy string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOrderBy = r.URL.Query().Get("order_by")
		_ = json.NewEncoder(w).Encode(paginatedResponse[Comment]{
			Results: []Comment{{ID: "c1", CommentHTML: "<p>hi</p>"}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	comments, err := c.ListComments(context.Background(), "ws", "proj", "item1")
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(comments) != 1 || comments[0].ID != "c1" {
		t.Fatalf("comments = %+v, want one comment c1", comments)
	}
	if gotOrderBy != "created_at" {
		t.Errorf("order_by = %q, want %q (oldest first)", gotOrderBy, "created_at")
	}
}

func TestCreateAndUpdateComment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			CommentHTML string `json:"comment_html"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		switch r.Method {
		case http.MethodPost:
			if body.CommentHTML != "<p>new</p>" {
				t.Errorf("create body = %q", body.CommentHTML)
			}
			_ = json.NewEncoder(w).Encode(Comment{ID: "c1", CommentHTML: body.CommentHTML})
		case http.MethodPatch:
			if body.CommentHTML != "<p>edited</p>" {
				t.Errorf("update body = %q", body.CommentHTML)
			}
			_ = json.NewEncoder(w).Encode(Comment{ID: "c1", CommentHTML: body.CommentHTML})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")

	created, err := c.CreateComment(context.Background(), "ws", "proj", "item1", "<p>new</p>")
	if err != nil {
		t.Fatalf("CreateComment: %v", err)
	}
	if created.ID != "c1" || created.CommentHTML != "<p>new</p>" {
		t.Fatalf("created = %+v", created)
	}

	edited, err := c.UpdateComment(context.Background(), "ws", "proj", "item1", "c1", "<p>edited</p>")
	if err != nil {
		t.Fatalf("UpdateComment: %v", err)
	}
	if edited.CommentHTML != "<p>edited</p>" {
		t.Fatalf("edited = %+v", edited)
	}
}

func TestCreateWorkItem(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(WorkItem{ID: "wi-1", Name: gotBody["name"].(string), State: gotBody["state"].(string)})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	item, err := c.CreateWorkItem(context.Background(), "ws", "proj", map[string]any{"name": "New card", "state": "s1"})
	if err != nil {
		t.Fatalf("CreateWorkItem: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if want := "/api/v1/workspaces/ws/projects/proj/work-items/"; gotPath != want {
		t.Errorf("path = %s, want %s", gotPath, want)
	}
	if item.ID != "wi-1" || item.Name != "New card" || item.State != "s1" {
		t.Fatalf("created = %+v", item)
	}
}

func TestListAttachments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if want := "/api/v1/workspaces/ws/projects/proj/work-items/item1/attachments/"; r.URL.Path != want {
			t.Errorf("path = %s, want %s", r.URL.Path, want)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "a1", "created_at": "2026-01-01T00:00:00Z", "attributes": map[string]any{"name": "f.pdf", "type": "application/pdf", "size": 1234}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	items, err := c.ListAttachments(context.Background(), "ws", "proj", "item1")
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if len(items) != 1 || items[0].Name() != "f.pdf" || items[0].Size() != 1234 {
		t.Fatalf("items = %+v", items)
	}
}

// TestCreateUploadConfirmAttachmentFlow drives the three-step attach-a-file flow end to end
// against two mock servers (the Plane API and, standing in for S3, the presigned upload/
// download target it hands back) — the same split docs/plane.sh's upload-asset relies on in
// production.
func TestCreateUploadConfirmAttachmentFlow(t *testing.T) {
	var s3 *httptest.Server
	var confirmedID string
	var confirmBody map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/workspaces/ws/projects/proj/work-items/item1/attachments/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "notes.txt" {
			t.Errorf("create body name = %v, want notes.txt", body["name"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"asset_id": "asset-1",
			"upload_data": map[string]any{
				"url":    s3.URL + "/upload",
				"fields": map[string]string{"key": "some-key", "policy": "p"},
			},
		})
	})
	mux.HandleFunc("/api/v1/workspaces/ws/projects/proj/work-items/item1/attachments/asset-1/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&confirmBody)
		confirmedID = "asset-1"
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var uploadedFields map[string]string
	var uploadedFileContent []byte
	s3mux := http.NewServeMux()
	s3mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		uploadedFields = map[string]string{}
		for k, v := range r.MultipartForm.Value {
			uploadedFields[k] = v[0]
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		uploadedFileContent, _ = io.ReadAll(file)
	})
	s3 = httptest.NewServer(s3mux)
	defer s3.Close()

	c := New(srv.URL, "tok")
	ctx := context.Background()

	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	assetID, uploadURL, fields, err := c.CreateAttachment(ctx, "ws", "proj", "item1", "notes.txt", "text/plain", 11)
	if err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}
	if assetID != "asset-1" || uploadURL != s3.URL+"/upload" {
		t.Fatalf("assetID=%q uploadURL=%q", assetID, uploadURL)
	}

	if err := c.UploadAttachmentFile(ctx, uploadURL, fields, path, "text/plain"); err != nil {
		t.Fatalf("UploadAttachmentFile: %v", err)
	}
	if string(uploadedFileContent) != "hello world" {
		t.Errorf("uploaded content = %q, want %q", uploadedFileContent, "hello world")
	}
	if uploadedFields["key"] != "some-key" || uploadedFields["policy"] != "p" {
		t.Errorf("uploaded fields = %v, want key=some-key policy=p", uploadedFields)
	}

	if err := c.ConfirmAttachmentUploaded(ctx, "ws", "proj", "item1", assetID); err != nil {
		t.Fatalf("ConfirmAttachmentUploaded: %v", err)
	}
	if confirmedID != "asset-1" || confirmBody["is_uploaded"] != true {
		t.Fatalf("confirmedID=%q confirmBody=%v, want asset-1/is_uploaded=true", confirmedID, confirmBody)
	}
}

// TestDownloadAttachmentFollowsRedirect checks DownloadAttachment follows the API's redirect to
// the presigned download URL and does not forward the API token onto it.
func TestDownloadAttachmentFollowsRedirect(t *testing.T) {
	var download *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/workspaces/ws/projects/proj/work-items/item1/attachments/asset-1/", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Api-Key"); got != "tok" {
			t.Errorf("X-Api-Key = %q, want tok", got)
		}
		http.Redirect(w, r, download.URL+"/file.bin", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	download = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Api-Key"); got != "" {
			t.Errorf("the presigned download request must not carry the API token, got %q", got)
		}
		_, _ = w.Write([]byte("file bytes"))
	}))
	defer download.Close()

	c := New(srv.URL, "tok")
	data, err := c.DownloadAttachment(context.Background(), "ws", "proj", "item1", "asset-1")
	if err != nil {
		t.Fatalf("DownloadAttachment: %v", err)
	}
	if string(data) != "file bytes" {
		t.Errorf("data = %q, want %q", data, "file bytes")
	}
}
