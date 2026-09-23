package tui

import (
	"testing"

	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// TestWorkItemURL checks the "u" (copy url) action builds the same "browse" route the web
// app's own issue detail page resolves — project identifier and sequence number joined by a
// dash, not the work item's own UUID (see workItemURL).
func TestWorkItemURL(t *testing.T) {
	m := Model{
		serverURL:     "https://app.plane.so",
		workspaceSlug: "acme",
		project:       api.Project{ID: "proj-uuid", Identifier: "ENG"},
	}
	it := api.WorkItem{ID: "item-uuid", SequenceID: 123}

	want := "https://app.plane.so/acme/browse/ENG-123"
	if got := m.workItemURL(it); got != want {
		t.Errorf("workItemURL = %q, want %q", got, want)
	}
}
