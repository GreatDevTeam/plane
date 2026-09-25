package tui

import (
	"strings"
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

// TestOpenItemInBrowser checks the "U" (shift+u) action, alongside "u"'s copy-to-clipboard,
// hands the same work item URL to the OS's default handler.
func TestOpenItemInBrowser(t *testing.T) {
	opened := stubOpenURL(t)
	m := Model{
		serverURL:     "https://app.plane.so",
		workspaceSlug: "acme",
		project:       api.Project{ID: "proj-uuid", Identifier: "ENG"},
	}
	it := api.WorkItem{ID: "item-uuid", SequenceID: 123}

	next, _ := m.openItemInBrowser(&it)
	m = next.(Model)

	want := "https://app.plane.so/acme/browse/ENG-123"
	if len(*opened) != 1 || (*opened)[0] != want {
		t.Errorf("opened = %v, want [%q]", *opened, want)
	}
	if !strings.Contains(m.status, want) {
		t.Errorf("status = %q, want it to mention the opened url", m.status)
	}
}

// TestOpenItemInBrowserNilItem checks the board's U with no card focused is a no-op rather
// than a nil-pointer panic.
func TestOpenItemInBrowserNilItem(t *testing.T) {
	opened := stubOpenURL(t)
	m := Model{}
	if _, cmd := m.openItemInBrowser(nil); cmd != nil {
		t.Error("openItemInBrowser(nil) returned a command")
	}
	if len(*opened) != 0 {
		t.Errorf("openItemInBrowser(nil) opened %v, want nothing", *opened)
	}
}
