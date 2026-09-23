package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

	items, cursor, hasNext, err := c.ListWorkItemsPage(context.Background(), "ws", "proj", "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(items) != 2 || cursor != "page2" || !hasNext {
		t.Fatalf("first page = %+v, %q, %v; want 2 items, cursor page2, hasNext true", items, cursor, hasNext)
	}

	items, cursor, hasNext, err = c.ListWorkItemsPage(context.Background(), "ws", "proj", cursor)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(items) != 1 || cursor != "" || hasNext {
		t.Fatalf("second page = %+v, %q, %v; want 1 item, no next", items, cursor, hasNext)
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
