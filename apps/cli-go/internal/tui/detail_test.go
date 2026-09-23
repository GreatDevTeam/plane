package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

func detailFixture() Model {
	item := api.WorkItem{
		ID:              "wi-1",
		SequenceID:      42,
		Name:            "Do the thing",
		State:           "s1",
		Priority:        "high",
		Assignees:       []string{"u1", "u2"},
		DescriptionHTML: "<p>first line</p><p><strong>second</strong> line</p>",
	}
	return Model{
		screen:     screenDetail,
		width:      120,
		height:     40,
		states:     []api.State{{ID: "s1", Name: "Todo"}},
		items:      []api.WorkItem{item},
		colCursor:  []int{0},
		detailItem: &item,
		members: []api.Member{
			{ID: "u1", FirstName: "Jane", LastName: "Doe", DisplayName: "jane.doe", Email: "jane@x.com"},
			{ID: "u2", DisplayName: "sam", Email: "sam@x.com"},
		},
		editor: newTestEditor(),
	}
}

// newTestEditor is the same textarea New() builds, without needing a whole Model.
func newTestEditor() textarea.Model {
	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.SetWidth(70)
	ta.SetHeight(5)
	return ta
}

// TestDetailShowsAssigneeFullNames covers "on task page show assignees as full names": the
// detail screen used to print the number of assignees and nothing else.
func TestDetailShowsAssigneeFullNames(t *testing.T) {
	m := detailFixture()
	view := m.viewDetail()
	if !strings.Contains(view, "Jane Doe") {
		t.Errorf("detail screen does not show the assignee's full name:\n%s", view)
	}
	if !strings.Contains(view, "sam") {
		t.Error("detail screen dropped an assignee with no first/last name")
	}
	if strings.Contains(view, "Assignees: 2") {
		t.Error("detail screen still shows the assignee count instead of names")
	}

	m.detailItem.Assignees = nil
	if !strings.Contains(m.viewDetail(), "Assignees: —") {
		t.Error("a work item with no assignees should say so")
	}
}

// TestDetailFooterShowsBackgroundLoader covers the footer loader for the lazy card open: the
// cached copy is on screen immediately and the footer says what is still being fetched.
func TestDetailFooterShowsBackgroundLoader(t *testing.T) {
	m := detailFixture()
	if m.detailLoader() != "" {
		t.Error("an idle detail screen must not claim to be refreshing")
	}

	m.detailLoading = true
	m.commentsLoading = true
	view := m.viewDetail()
	if !strings.Contains(view, "refreshing work item and comments") {
		t.Errorf("footer does not report the background refresh:\n%s", view)
	}
	if !strings.Contains(view, "Do the thing") {
		t.Error("the cached work item is not shown while it refreshes")
	}
}

// TestOpenCardRefreshesInTheBackground checks enter on the board opens the cached card and
// asks for a fresh copy of both the item and its comments.
func TestOpenCardRefreshesInTheBackground(t *testing.T) {
	m := boardFixture(2, 10, 120, 40)
	m.client = api.New("http://example.invalid", "token")
	m.project = api.Project{ID: "proj-1"}
	m.comments = []api.Comment{{ID: "stale"}}

	next, cmd := m.updateBoard(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.screen != screenDetail {
		t.Fatal("enter did not open the detail screen")
	}
	if m.detailItem == nil {
		t.Fatal("the detail screen opened without the cached work item")
	}
	if !m.detailLoading || !m.commentsLoading {
		t.Errorf("opening a card did not start a background refresh (item=%v comments=%v)",
			m.detailLoading, m.commentsLoading)
	}
	if len(m.comments) != 0 {
		t.Error("the previous card's comments are still on screen")
	}
	if cmd == nil {
		t.Error("opening a card issued no fetch")
	}
}

// TestHandleCommentsFocusesTheLatestComment checks the detail screen opens with the cursor on
// the most recent comment (comments render oldest first, so that is the last one), not the
// oldest, and that the focused comment's header renders with the lighter focused style.
func TestHandleCommentsFocusesTheLatestComment(t *testing.T) {
	m := detailFixture()
	m.commentCursor = 0
	next, _ := m.handleComments(commentsMsg{items: []api.Comment{
		{ID: "c1", Actor: "u1", CreatedAt: "2026-01-01T00:00:00Z"},
		{ID: "c2", Actor: "u2", CreatedAt: "2026-01-02T00:00:00Z"},
		{ID: "c3", Actor: "u1", CreatedAt: "2026-01-03T00:00:00Z"},
	}})
	m = next.(Model)
	if m.commentCursor != 2 {
		t.Fatalf("commentCursor = %d, want 2 (the latest comment)", m.commentCursor)
	}
	view := m.viewComments()
	lines := strings.Split(view, "\n")
	var focusedLine string
	for _, l := range lines {
		if strings.Contains(l, "> ") {
			focusedLine = l
		}
	}
	if !strings.Contains(focusedLine, m.memberName("u1")) {
		t.Errorf("focused comment line %q is not the latest comment (by u1)", focusedLine)
	}
}

// TestHandleCommentsOnEmptyList checks an empty comment thread does not leave the cursor
// pointing at a non-existent comment (len-1 == -1 must stay a safe, unused value).
func TestHandleCommentsOnEmptyList(t *testing.T) {
	m := detailFixture()
	next, _ := m.handleComments(commentsMsg{items: nil})
	m = next.(Model)
	if m.commentCursor != -1 {
		t.Fatalf("commentCursor = %d, want -1 for an empty comment list", m.commentCursor)
	}
	if strings.Contains(m.viewComments(), "> ") {
		t.Error("an empty comment list rendered a focus marker")
	}
}

// TestHandleWorkItemLoadedIgnoresAStaleAnswer covers the race the lazy open creates: the user
// can be looking at another card by the time a slow fetch lands.
func TestHandleWorkItemLoadedIgnoresAStaleAnswer(t *testing.T) {
	m := detailFixture()
	other := api.WorkItem{ID: "wi-2", Name: "another card"}
	m.detailItem = &other
	m.detailLoading = true

	fresh := api.WorkItem{ID: "wi-1", Name: "renamed on the server"}
	next, _ := m.handleWorkItemLoaded(workItemLoadedMsg{item: &fresh})
	m = next.(Model)

	if m.detailItem.ID != "wi-2" {
		t.Error("a late answer for another card replaced the one on screen")
	}
	if m.items[0].Name != "renamed on the server" {
		t.Error("the board's cached copy was not updated by the background fetch")
	}
	if m.detailLoading {
		t.Error("the footer loader was left on")
	}
}

// TestDescriptionEditor covers editing a work item description: d opens the editor seeded
// with the current description as plain text, and ctrl+s sends it back as HTML.
func TestDescriptionEditor(t *testing.T) {
	m := detailFixture()
	m.client = api.New("http://example.invalid", "token")
	m.project = api.Project{ID: "proj-1"}

	next, _ := m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = next.(Model)
	if !m.editorOn || m.editorMode != "description" {
		t.Fatalf("d did not open the description editor (on=%v mode=%q)", m.editorOn, m.editorMode)
	}
	if got := m.editor.Value(); !strings.Contains(got, "first line") || strings.Contains(got, "<p>") {
		t.Errorf("editor seeded with %q, want the description as plain text", got)
	}
	if !strings.Contains(m.viewDetail(), "Edit description") {
		t.Error("the editor is not labelled as a description edit")
	}

	m.editor.SetValue("rewritten\n\nsecond paragraph")
	next, cmd := m.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("ctrl+s did not save the description")
	}
	if !strings.Contains(m.status, "Saving description") {
		t.Errorf("status = %q, want it to mention saving the description", m.status)
	}

	// The save comes back as a normal work item update, which must close the editor.
	saved := api.WorkItem{ID: "wi-1", Name: "Do the thing", DescriptionHTML: "<p>rewritten</p>"}
	next, _ = m.handleWorkItemUpdated(workItemUpdatedMsg{item: &saved})
	m = next.(Model)
	if m.editorOn {
		t.Error("the description editor stayed open after the save came back")
	}
	if m.detailItem.DescriptionHTML != "<p>rewritten</p>" {
		t.Error("the detail screen is still showing the old description")
	}
}

// TestDescriptionEditorCanClearADescription: an empty description is a real edit, unlike an
// empty comment, which is silently discarded.
func TestDescriptionEditorCanClearADescription(t *testing.T) {
	m := detailFixture()
	m.client = api.New("http://example.invalid", "token")
	m.project = api.Project{ID: "proj-1"}

	next, _ := m.openDescriptionEditor()
	m = next.(Model)
	m.editor.SetValue("")
	if _, cmd := m.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlS}); cmd == nil {
		t.Error("clearing a description was discarded like an empty comment")
	}

	next, _ = m.openCommentEditor("")
	m = next.(Model)
	m.editor.SetValue("   ")
	if _, cmd := m.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlS}); cmd != nil {
		t.Error("an empty comment was posted")
	}
}

// TestEditorEscLeavesNoState checks esc drops the draft and the mode with it, so the next
// thing opened is not pre-filled with the last one.
func TestEditorEscLeavesNoState(t *testing.T) {
	m := detailFixture()
	next, _ := m.openDescriptionEditor()
	m = next.(Model)
	next, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.editorOn || m.editorMode != "" || m.editor.Value() != "" {
		t.Errorf("esc left the editor behind (on=%v mode=%q value=%q)", m.editorOn, m.editorMode, m.editor.Value())
	}
}
