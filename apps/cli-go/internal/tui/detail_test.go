package tui

import (
	"fmt"
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
		editor:          newTestEditor(),
		pickerSearch:    newInput("", 40),
		attachPathInput: newInput("", 60),
		colorInput:      newInput("", 10),
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

// TestDetailManualRefresh checks "r" on the detail screen refreshes both the board (which is
// what keeps the open item's own fields in sync — see applyBoardRefresh) and this item's
// comments, and that a second "r" while one is already in flight is a no-op rather than
// stacking a second request.
func TestDetailManualRefresh(t *testing.T) {
	m := detailFixture()
	m.client = api.New("http://example.invalid", "token")
	m.project = api.Project{ID: "proj-1"}

	next, cmd := m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("r did not issue a refresh")
	}
	if !m.refreshing || !m.commentsLoading {
		t.Errorf("r did not mark a refresh in flight (refreshing=%v commentsLoading=%v)", m.refreshing, m.commentsLoading)
	}
	if !strings.Contains(m.status, "Refreshing") {
		t.Errorf("status = %q, want it to mention refreshing", m.status)
	}

	_, cmd = m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd != nil {
		t.Error("r while already refreshing issued a second request")
	}
}

// TestBoardTickRefreshesDetailComments checks the 30s auto-refresh (handleBoardTick) also
// re-fetches comments while the detail screen is open — applyBoardRefresh already keeps the
// item's own fields in sync, but comments are a separate endpoint it never touches.
func TestBoardTickRefreshesDetailComments(t *testing.T) {
	m := detailFixture()
	m.client = api.New("http://example.invalid", "token")
	m.project = api.Project{ID: "proj-1"}

	next, cmd := m.handleBoardTick(boardTickMsg{})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("the tick issued no commands")
	}
	if !m.refreshing || !m.commentsLoading {
		t.Errorf("the tick did not start a refresh (refreshing=%v commentsLoading=%v)", m.refreshing, m.commentsLoading)
	}
}

// TestHandleCommentsFocusesTheLatestComment checks the detail screen opens with the cursor on
// the most recent comment (comments render oldest first, so that is the last one), not the
// oldest, and that the focused comment's header renders with the lighter focused style.
func TestHandleCommentsFocusesTheLatestComment(t *testing.T) {
	m := detailFixture()
	m.commentCursor = 0
	next, _ := m.handleComments(commentsMsg{workItemID: "wi-1", items: []api.Comment{
		{ID: "c1", Actor: "u1", CreatedAt: "2026-01-01T00:00:00Z"},
		{ID: "c2", Actor: "u2", CreatedAt: "2026-01-02T00:00:00Z"},
		{ID: "c3", Actor: "u1", CreatedAt: "2026-01-03T00:00:00Z"},
	}})
	m = next.(Model)
	if m.commentCursor != 2 {
		t.Fatalf("commentCursor = %d, want 2 (the latest comment)", m.commentCursor)
	}
	view, _, _ := m.commentsContent(m.width)
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

// TestHandleCommentsIgnoresStaleWorkItem covers the "no comments yet but the task sure has
// comments" bug: opening a card starts a comment fetch, and if the user backs out and opens a
// different one before that fetch lands, the response is for a work item the screen has since
// moved on from. Applying it anyway used to overwrite the newly opened item's (possibly
// non-empty) comment thread with the previous item's — including an empty one, which rendered
// as "No comments yet." for an item that actually has some.
func TestHandleCommentsIgnoresStaleWorkItem(t *testing.T) {
	m := detailFixture() // detailItem.ID == "wi-1"
	m.comments = []api.Comment{{ID: "real", Actor: "u1", CreatedAt: "2026-01-01T00:00:00Z"}}
	m.commentsLoading = true

	next, _ := m.handleComments(commentsMsg{workItemID: "wi-2", items: nil})
	m = next.(Model)

	if len(m.comments) != 1 || m.comments[0].ID != "real" {
		t.Fatalf("a stale response for a different work item overwrote the current comments: %+v", m.comments)
	}
	if !m.commentsLoading {
		t.Error("a stale response cleared commentsLoading for the still-in-flight current fetch")
	}
}

// TestHandleCommentsOnEmptyList checks an empty comment thread does not leave the cursor
// pointing at a non-existent comment (len-1 == -1 must stay a safe, unused value).
func TestHandleCommentsOnEmptyList(t *testing.T) {
	m := detailFixture()
	next, _ := m.handleComments(commentsMsg{workItemID: "wi-1", items: nil})
	m = next.(Model)
	if m.commentCursor != -1 {
		t.Fatalf("commentCursor = %d, want -1 for an empty comment list", m.commentCursor)
	}
	view, _, _ := m.commentsContent(m.width)
	if strings.Contains(view, "> ") {
		t.Error("an empty comment list rendered a focus marker")
	}
}

// TestDeleteSelectedCommentRejectsSomeoneElses covers "x" (delete comment): only the comment's
// own author may delete it, the same restriction "e" (edit) already enforces, and this must be
// checked locally before ever firing the request.
func TestDeleteSelectedCommentRejectsSomeoneElses(t *testing.T) {
	m := detailFixture()
	m.client = api.New("http://example.invalid", "token")
	m.project = api.Project{ID: "proj-1"}
	m.user = &api.User{ID: "u1"}
	next, _ := m.handleComments(commentsMsg{workItemID: "wi-1", items: []api.Comment{
		{ID: "c1", Actor: "someone-else", CreatedAt: "2026-01-01T00:00:00Z", CommentHTML: "<p>hi</p>"},
	}})
	m = next.(Model)

	next, cmd := m.deleteSelectedComment()
	m = next.(Model)
	if cmd != nil {
		t.Fatal("deleting someone else's comment fired a request")
	}
	if m.err == "" {
		t.Fatal("deleting someone else's comment did not surface an error")
	}
	if len(m.comments) != 1 {
		t.Fatal("the comment was removed locally despite being rejected")
	}
}

// TestHandleCommentDeletedRemovesCommentAndMovesCursor covers the delete landing: the deleted
// comment must disappear from m.comments (no confirmation round-trip, it is just gone), and the
// cursor must land on the comment that took its place rather than past the end of the list.
func TestHandleCommentDeletedRemovesCommentAndMovesCursor(t *testing.T) {
	m := detailFixture()
	m.user = &api.User{ID: "u1"}
	next, _ := m.handleComments(commentsMsg{workItemID: "wi-1", items: []api.Comment{
		{ID: "c1", Actor: "u1", CreatedAt: "2026-01-01T00:00:00Z", CommentHTML: "<p>one</p>"},
		{ID: "c2", Actor: "u1", CreatedAt: "2026-01-02T00:00:00Z", CommentHTML: "<p>two</p>"},
	}})
	m = next.(Model)
	m.commentCursor = 1 // on c2, the last comment

	next, _ = m.handleCommentDeleted(commentDeletedMsg{commentID: "c2"})
	m = next.(Model)
	if len(m.comments) != 1 || m.comments[0].ID != "c1" {
		t.Fatalf("comments after delete = %+v, want only c1 left", m.comments)
	}
	if m.commentCursor != 0 {
		t.Errorf("commentCursor = %d, want 0 after deleting the last comment", m.commentCursor)
	}
}

// TestHandleCommentsBackgroundRefreshNoOpsWhenUnchanged covers the auto-refresh blink: the 30s
// tick re-fetches comments unconditionally (TestBoardTickRefreshesDetailComments), and if the
// thread has not actually changed, handleComments must leave the cursor and the rendered
// content exactly as they were rather than replacing everything (which is what made the pane
// blink even on a no-op refresh).
func TestHandleCommentsBackgroundRefreshNoOpsWhenUnchanged(t *testing.T) {
	m := detailFixture()
	same := []api.Comment{
		{ID: "c1", Actor: "u1", CreatedAt: "2026-01-01T00:00:00Z", CommentHTML: "<p>hi</p>"},
		{ID: "c2", Actor: "u2", CreatedAt: "2026-01-02T00:00:00Z", CommentHTML: "<p>hey</p>"},
	}
	next, _ := m.handleComments(commentsMsg{workItemID: "wi-1", items: same})
	m = next.(Model)
	m.commentCursor = 0 // simulate the user having scrolled back to the first comment

	// A background refresh (comments already loaded) with an identical thread.
	unchanged := []api.Comment{
		{ID: "c1", Actor: "u1", CreatedAt: "2026-01-01T00:00:00Z", CommentHTML: "<p>hi</p>"},
		{ID: "c2", Actor: "u2", CreatedAt: "2026-01-02T00:00:00Z", CommentHTML: "<p>hey</p>"},
	}
	next, _ = m.handleComments(commentsMsg{workItemID: "wi-1", items: unchanged})
	m = next.(Model)
	if m.commentCursor != 0 {
		t.Errorf("an unchanged background refresh moved the cursor to %d, want it left at 0", m.commentCursor)
	}
}

// TestHandleCommentsBackgroundRefreshKeepsCursorOnSameComment covers a real change arriving in
// the background (a new comment posted elsewhere): the cursor must stay on the same comment the
// user was reading, not jump to the newest one the way the very first load does.
func TestHandleCommentsBackgroundRefreshKeepsCursorOnSameComment(t *testing.T) {
	m := detailFixture()
	first := []api.Comment{
		{ID: "c1", Actor: "u1", CreatedAt: "2026-01-01T00:00:00Z"},
		{ID: "c2", Actor: "u2", CreatedAt: "2026-01-02T00:00:00Z"},
	}
	next, _ := m.handleComments(commentsMsg{workItemID: "wi-1", items: first})
	m = next.(Model)
	m.commentCursor = 0 // reading the first comment when a new one arrives

	withNewComment := []api.Comment{
		{ID: "c1", Actor: "u1", CreatedAt: "2026-01-01T00:00:00Z"},
		{ID: "c2", Actor: "u2", CreatedAt: "2026-01-02T00:00:00Z"},
		{ID: "c3", Actor: "u1", CreatedAt: "2026-01-03T00:00:00Z"},
	}
	next, _ = m.handleComments(commentsMsg{workItemID: "wi-1", items: withNewComment})
	m = next.(Model)
	if m.commentCursor != 0 || m.comments[m.commentCursor].ID != "c1" {
		t.Errorf("commentCursor = %d (%q), want it to stay on c1", m.commentCursor, m.comments[m.commentCursor].ID)
	}
}

// TestCommentsContentDoesNotBlankDuringBackgroundRefresh covers the other half of the blink:
// commentsLoading flips true the instant the 30s tick fires a re-fetch (handleBoardTick), but
// the thread already on screen must stay visible until handleComments actually has something
// new — not get replaced by the loading placeholder every tick.
func TestCommentsContentDoesNotBlankDuringBackgroundRefresh(t *testing.T) {
	m := detailFixture()
	next, _ := m.handleComments(commentsMsg{workItemID: "wi-1", items: []api.Comment{
		{ID: "c1", Actor: "u1", CreatedAt: "2026-01-01T00:00:00Z", CommentHTML: "<p>hi there</p>"},
	}})
	m = next.(Model)
	m.commentsLoading = true // a background refresh is now in flight

	view, _, _ := m.commentsContent(m.width)
	if strings.Contains(view, "Loading comments") {
		t.Error("a background refresh blanked the comments pane with the loading placeholder")
	}
	if !strings.Contains(view, "hi there") {
		t.Errorf("the existing comment disappeared while a background refresh was in flight: %q", view)
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

// TestHandleWorkItemLoadedSkipsAnswerOlderThanALaterPatch covers the race behind "changes made
// from the task view don't stick": opening a card fires a background GET (openWorkItem) that
// can still be in flight when the user immediately changes state/priority/assignee/labels from
// a picker. That PATCH's own response (handleWorkItemUpdated) applies first and is
// authoritative; the slower GET must not then overwrite it with the pre-change data it fetched.
func TestHandleWorkItemLoadedSkipsAnswerOlderThanALaterPatch(t *testing.T) {
	m := detailFixture()
	m.items[0].State = "s2"
	m.items[0].UpdatedAt = "2026-01-02T00:00:00Z"
	m.detailItem.State = "s2"
	m.detailItem.UpdatedAt = "2026-01-02T00:00:00Z"

	stale := api.WorkItem{ID: "wi-1", State: "s1", UpdatedAt: "2026-01-01T00:00:00Z"}
	next, _ := m.handleWorkItemLoaded(workItemLoadedMsg{item: &stale})
	m = next.(Model)

	if m.items[0].State != "s2" {
		t.Errorf("board state = %q, a GET older than the last PATCH reverted it", m.items[0].State)
	}
	if m.detailItem.State != "s2" {
		t.Errorf("detail state = %q, a GET older than the last PATCH reverted it", m.detailItem.State)
	}
}

// TestCommentsContentIncludesActivities covers "load all actions as well (created, state
// changed, etc)": the comments pane must also show the work item's activity log (activities.go),
// interleaved with comments in timestamp order, even though only comments are ever focusable —
// commentCursor still indexes m.comments alone.
func TestCommentsContentIncludesActivities(t *testing.T) {
	m := detailFixture()
	m.comments = []api.Comment{
		{ID: "c1", Actor: "u1", CreatedAt: "2026-01-01T00:00:00Z", CommentHTML: "<p>hello</p>"},
	}
	m.commentCursor = 0
	m.activities = []api.Activity{
		// Oldest first, matching the API's own order_by=created_at (see ListActivities) —
		// the merge below assumes both slices already arrive sorted this way.
		{ID: "a2", Actor: "u1", CreatedAt: "2025-12-31T00:00:00Z", Comment: "created the issue"},
		{ID: "a1", Actor: "u2", CreatedAt: "2026-01-02T00:00:00Z", Field: "state", OldValue: "Todo", NewValue: "In Progress"},
	}

	view, _, _ := m.commentsContent(m.width)
	if !strings.Contains(view, "hello") {
		t.Errorf("comment missing from the merged view: %q", view)
	}
	if !strings.Contains(view, "changed state from Todo to In Progress") {
		t.Errorf("state-change activity missing from the merged view: %q", view)
	}
	if !strings.Contains(view, "created the issue") {
		t.Errorf("creation activity missing from the merged view: %q", view)
	}
	// a2 (2025-12-31) predates c1 (2026-01-01), which predates a1 (2026-01-02): the merge must
	// preserve that order rather than grouping all activities after all comments.
	if i, j := strings.Index(view, "created the issue"), strings.Index(view, "hello"); i > j {
		t.Errorf("activity a2 rendered after comment c1 despite its earlier timestamp: %q", view)
	}
	if i, j := strings.Index(view, "hello"), strings.Index(view, "changed state"); i > j {
		t.Errorf("comment c1 rendered after activity a1 despite its earlier timestamp: %q", view)
	}
}

// TestDetailHeaderShowsActivityCount checks the pane header names both counts once there is at
// least one activity entry, rather than only ever showing the comment count.
func TestDetailHeaderShowsActivityCount(t *testing.T) {
	m := detailFixture()
	m.comments = []api.Comment{{ID: "c1", Actor: "u1", CreatedAt: "2026-01-01T00:00:00Z"}}
	m.activities = []api.Activity{{ID: "a1", Actor: "u1", CreatedAt: "2026-01-02T00:00:00Z", Field: "priority", NewValue: "high"}}
	m.commentCursor = 0

	if view := m.viewDetail(); !strings.Contains(view, "Activity (1)") {
		t.Errorf("header did not show the activity count: %q", view)
	}
}

// TestDescriptionEditor covers editing a work item description: d opens the editor seeded
// with the current description as plain text, and enter sends it back as HTML.
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
	next, cmd := m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("enter did not save the description")
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
	if _, cmd := m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil {
		t.Error("clearing a description was discarded like an empty comment")
	}

	next, _ = m.openCommentEditor("")
	m = next.(Model)
	m.editor.SetValue("   ")
	if _, cmd := m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Error("an empty comment was posted")
	}
}

// TestCommentEditorEnterSubmitsAltEnterInsertsNewline covers the comment/new-item editor's
// submit key: plain enter saves, and alt+enter inserts a literal newline instead of submitting,
// the portable substitute for ctrl+enter/shift+enter this bubbletea version cannot detect on a
// standard terminal (see updateEditor's doc comment). The description editor uses the same
// scheme — see TestDescriptionEditor.
func TestCommentEditorEnterSubmitsAltEnterInsertsNewline(t *testing.T) {
	m := detailFixture()
	m.client = api.New("http://example.invalid", "token")
	m.project = api.Project{ID: "proj-1"}

	next, _ := m.openCommentEditor("")
	m = next.(Model)
	m.editor.SetValue("first line")

	next, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m = next.(Model)
	if !m.editorOn {
		t.Fatal("alt+enter closed the editor instead of inserting a newline")
	}
	if got := m.editor.Value(); got != "first line\n" {
		t.Fatalf("editor value after alt+enter = %q, want a trailing newline appended", got)
	}

	m.editor.SetValue("second line")
	next, cmd := m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("enter did not submit the comment")
	}
	if !strings.Contains(m.status, "Saving comment") {
		t.Errorf("status = %q, want it to mention saving the comment", m.status)
	}
}

// TestDescriptionEditorAltEnterAndCtrlJInsertNewline locks in the consolidated shortcut scheme
// for the description editor: it used to treat plain enter as a newline and ctrl+s as save (see
// TestDescriptionEditor's history); now, like the comment editor, alt+enter and ctrl+j both
// insert a newline instead of saving.
func TestDescriptionEditorAltEnterAndCtrlJInsertNewline(t *testing.T) {
	m := detailFixture()
	m.client = api.New("http://example.invalid", "token")
	m.project = api.Project{ID: "proj-1"}

	next, _ := m.openDescriptionEditor()
	m = next.(Model)
	m.editor.SetValue("first line")

	next, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m = next.(Model)
	if !m.editorOn {
		t.Fatal("alt+enter closed the description editor instead of inserting a newline")
	}
	if got := m.editor.Value(); got != "first line\n" {
		t.Fatalf("editor value after alt+enter = %q, want a trailing newline appended", got)
	}

	next, _ = m.updateEditor(tea.KeyMsg{Type: tea.KeyCtrlJ})
	m = next.(Model)
	if !m.editorOn {
		t.Fatal("ctrl+j closed the description editor instead of inserting a newline")
	}
	if got := m.editor.Value(); got != "first line\n\n" {
		t.Fatalf("editor value after ctrl+j = %q, want a second trailing newline appended", got)
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

// longDetailFixture builds a detailFixture with a long, multi-paragraph description and a
// long comment thread — enough of both to overflow either pane at every size the tests below
// use — sized to width x height.
func longDetailFixture(width, height int) Model {
	m := detailFixture()
	m.width, m.height = width, height

	var desc strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&desc, "<p>%s paragraph %d of the description, long enough to wrap on a narrow terminal.</p>",
			longNames[i%len(longNames)], i)
	}
	m.detailItem.DescriptionHTML = desc.String()

	comments := make([]api.Comment, 30)
	for i := range comments {
		comments[i] = api.Comment{
			ID:          fmt.Sprintf("c%d", i),
			Actor:       "u1",
			CreatedAt:   "2026-01-01T00:00:00Z",
			CommentHTML: fmt.Sprintf("<p>%s comment %d, also long enough to wrap on its own.</p>", longNames[i%len(longNames)], i),
		}
	}
	next, _ := m.handleComments(commentsMsg{workItemID: "wi-1", items: comments})
	return next.(Model)
}

// TestViewDetailFitsTerminal is the equivalent of TestViewBoardFitsTerminal for the detail
// screen: before descViewport/commentsViewport existed, the screen concatenated the header,
// description and every comment into one flat string with nothing to clip or scroll it, so a
// long description or comment thread simply overflowed past the terminal height. The two
// panes plus the header/meta/footer chrome around them must fit m.height and m.width exactly.
func TestViewDetailFitsTerminal(t *testing.T) {
	sizes := []struct{ width, height int }{
		{230, 52}, // full screen, 1080p
		{200, 50},
		{160, 48},
		{120, 40},
		{100, 30},
		{80, 24}, // the classic default
		{60, 20},
		{40, 12},
	}
	for _, size := range sizes {
		m := longDetailFixture(size.width, size.height)
		rows, cols := frameSize(m.viewDetail())
		if rows > size.height {
			t.Errorf("%dx%d terminal: frame is %d rows tall, want <= %d", size.width, size.height, rows, size.height)
		}
		if cols > size.width {
			t.Errorf("%dx%d terminal: frame is %d columns wide, want <= %d", size.width, size.height, cols, size.width)
		}
	}
}

// TestViewDetailFitsTerminalWithOverlays checks the same invariant with the description
// editor open and with a picker open, both of which push the two panes' row budget down —
// detailLayout must account for whichever is open, not just the plain footer.
func TestViewDetailFitsTerminalWithOverlays(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*Model)
	}{
		{"editor open", func(m *Model) { next, _ := m.openDescriptionEditor(); *m = next.(Model) }},
		{"picker open", func(m *Model) { m.openStatePicker() }},
	} {
		m := longDetailFixture(100, 30)
		tc.setup(&m)
		rows, cols := frameSize(m.viewDetail())
		if rows > 30 {
			t.Errorf("%s: frame is %d rows tall, want <= 30", tc.name, rows)
		}
		if cols > 100 {
			t.Errorf("%s: frame is %d columns wide, want <= 100", tc.name, cols)
		}
	}
}

// TestDetailTabSwitchesPaneFocus checks tab toggles which pane j/k drives: within the
// comments pane j/k scroll the pane one line at a time, independently of the comment cursor
// (n/p — see TestDetailNextPrevCommentJumpsFocus — are what move that), and once tab moves
// focus to the description pane the same keys scroll it instead.
func TestDetailTabSwitchesPaneFocus(t *testing.T) {
	m := longDetailFixture(80, 24)
	if m.detailFocus != detailPaneComments {
		t.Fatalf("detailFocus = %v, want detailPaneComments by default", m.detailFocus)
	}
	// longDetailFixture opens with the comments pane already scrolled to the bottom (the most
	// recent comment focused), so k (scroll up) is what has room to move — j is exercised in
	// the opposite direction below, once tab has moved focus away and back.
	cursorBefore := m.commentCursor
	commentsOffsetBefore := m.commentsViewport.YOffset

	next, _ := m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = next.(Model)
	if m.commentCursor != cursorBefore {
		t.Fatal("k moved the comment cursor while the comments pane was focused; it should only scroll")
	}
	if m.commentsViewport.YOffset >= commentsOffsetBefore {
		t.Errorf("k did not scroll the comments pane up (offset %d -> %d)", commentsOffsetBefore, m.commentsViewport.YOffset)
	}

	next, _ = m.updateDetail(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.detailFocus != detailPaneDescription {
		t.Fatalf("tab did not switch focus to the description pane (focus=%v)", m.detailFocus)
	}

	cursorBefore = m.commentCursor
	offsetBefore := m.descViewport.YOffset
	next, _ = m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = next.(Model)
	if m.commentCursor != cursorBefore {
		t.Error("j moved the comment cursor while the description pane was focused")
	}
	if m.descViewport.YOffset <= offsetBefore {
		t.Errorf("j did not scroll the description pane down (offset %d -> %d)", offsetBefore, m.descViewport.YOffset)
	}

	next, _ = m.updateDetail(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(Model)
	if m.detailFocus != detailPaneComments {
		t.Fatalf("a second tab did not switch focus back to the comments pane (focus=%v)", m.detailFocus)
	}
}

// TestDetailNextPrevCommentJumpsFocus checks n/p move commentCursor and scroll the newly
// focused comment fully into view, the way j/k used to before they became a plain one-line
// scroll (see TestDetailTabSwitchesPaneFocus).
func TestDetailNextPrevCommentJumpsFocus(t *testing.T) {
	m := longDetailFixture(80, 24)
	m.commentCursor = 0

	next, _ := m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = next.(Model)
	if m.commentCursor != 1 {
		t.Fatalf("n moved commentCursor to %d, want 1", m.commentCursor)
	}

	next, _ = m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = next.(Model)
	if m.commentCursor != 0 {
		t.Fatalf("p moved commentCursor to %d, want 0", m.commentCursor)
	}

	// p at the first comment, and n at the last, must not run past either end.
	next, _ = m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = next.(Model)
	if m.commentCursor != 0 {
		t.Fatalf("p at the first comment moved commentCursor to %d, want 0", m.commentCursor)
	}
	m.commentCursor = len(m.comments) - 1
	next, _ = m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = next.(Model)
	if m.commentCursor != len(m.comments)-1 {
		t.Fatalf("n at the last comment moved commentCursor to %d, want %d", m.commentCursor, len(m.comments)-1)
	}
}

// TestScrollCommentsToCursorKeepsLatestCommentVisible covers the notes on #2114: comments
// already focus the latest comment on open, but before the comments pane could scroll on its
// own that comment could still be cursor-selected far below the visible viewport. Once the
// thread is longer than the pane, opening it must scroll so the latest comment is actually
// on screen.
func TestScrollCommentsToCursorKeepsLatestCommentVisible(t *testing.T) {
	m := longDetailFixture(80, 20)
	_, _, focusEnd := m.commentsContent(m.commentsViewport.Width)
	if focusEnd < m.commentsViewport.YOffset || focusEnd > m.commentsViewport.YOffset+m.commentsViewport.Height-1 {
		t.Errorf("latest comment (line %d) is not within the visible window [%d, %d)",
			focusEnd, m.commentsViewport.YOffset, m.commentsViewport.YOffset+m.commentsViewport.Height)
	}
}
