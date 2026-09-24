package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
	"github.com/makeplane/plane/apps/cli-go/internal/config"
)

// relationsFixture is a board holding one three-generation family — a parent (#100), the
// item in the middle (#200, opened on the detail screen) and its sub-tasks (#300…) — plus an
// unrelated top-level item, so a test can tell "has no relations" from "relations not shown".
func relationsFixture(subTasks int) Model {
	states := []api.State{
		{ID: "s1", Name: "Todo"},
		{ID: "s2", Name: "In Progress"},
		{ID: "s3", Name: "Done"},
	}
	parent := api.WorkItem{ID: "p", SequenceID: 100, Name: "The parent item", State: "s3", Priority: "low"}
	item := api.WorkItem{ID: "wi", SequenceID: 200, Name: "The item on screen", State: "s2", Priority: "high", Parent: "p"}
	loner := api.WorkItem{ID: "lone", SequenceID: 900, Name: "Unrelated item", State: "s1", Priority: "none"}

	items := []api.WorkItem{parent, item, loner}
	// Appended in descending work item number, so a test that sees them in ascending order
	// is seeing subIssues sort them rather than the order they happened to arrive in.
	for i := subTasks - 1; i >= 0; i-- {
		items = append(items, api.WorkItem{
			ID:         fmt.Sprintf("c%d", i),
			SequenceID: 300 + i,
			Name:       fmt.Sprintf("Sub-task number %d", i),
			State:      states[i%len(states)].ID,
			Priority:   api.Priorities[i%len(api.Priorities)],
			Parent:     "wi",
		})
	}

	return Model{
		screen:       screenDetail,
		width:        120,
		height:       40,
		project:      api.Project{ID: "proj", Name: "Plane"},
		states:       states,
		items:        items,
		colCursor:    make([]int, len(states)),
		detailItem:   &item,
		editor:       newTestEditor(),
		pickerSearch: newInput("", 40),
	}
}

// TestSubIssuesDerivedFromTheBoardsItems covers the API gap this feature is built around: the
// REST API sends `parent` but never a work item's children, so sub-tasks are whatever items
// on the board point back at it — in work item number order, whatever order they arrived in.
func TestSubIssuesDerivedFromTheBoardsItems(t *testing.T) {
	m := relationsFixture(3)

	subs := m.subIssues("wi")
	if len(subs) != 3 {
		t.Fatalf("subIssues = %d items, want 3", len(subs))
	}
	for i, want := range []int{300, 301, 302} {
		if subs[i].SequenceID != want {
			t.Errorf("subIssues[%d] = #%d, want #%d (ordered by work item number)", i, subs[i].SequenceID, want)
		}
	}
	if n := m.subIssueCount("wi"); n != 3 {
		t.Errorf("subIssueCount = %d, want 3", n)
	}
	if subs := m.subIssues("lone"); len(subs) != 0 {
		t.Errorf("an item with no children reported %d sub-tasks", len(subs))
	}
	if subs := m.subIssues(""); subs != nil {
		t.Error(`subIssues("") must not treat every parentless item as its child`)
	}

	parent := m.parentOf(*m.detailItem)
	if parent == nil || parent.SequenceID != 100 {
		t.Fatalf("parentOf = %+v, want #100", parent)
	}
	if p := m.parentOf(*parent); p != nil {
		t.Errorf("a top-level item reported a parent: %+v", p)
	}
}

// TestChildCountsMatchesTheScan checks the per-frame map viewBoard precomputes agrees with
// the fallback scan subIssueCount does without it — the two must never disagree, or a card
// would show a different badge on the board than anywhere else.
func TestChildCountsMatchesTheScan(t *testing.T) {
	m := relationsFixture(4)
	scanned := make(map[string]int)
	for _, it := range m.items {
		if n := m.subIssueCount(it.ID); n > 0 {
			scanned[it.ID] = n
		}
	}

	m.subCounts = m.childCounts()
	if len(m.subCounts) != len(scanned) {
		t.Fatalf("childCounts has %d entries, the scan found %d", len(m.subCounts), len(scanned))
	}
	for id, want := range scanned {
		if got := m.subIssueCount(id); got != want {
			t.Errorf("%s: cached count %d, scanned count %d", id, got, want)
		}
	}
}

// TestCardShowsParentAndSubTaskBadges covers "show a card's parent/sub-tasks on its card":
// the badge has to be on the card and, just as importantly, the card must still be exactly
// cardRows rows tall — the board's whole row budget is built on that.
func TestCardShowsParentAndSubTaskBadges(t *testing.T) {
	m := relationsFixture(3)
	m.subCounts = m.childCounts()

	byID := func(id string) api.WorkItem {
		for _, it := range m.items {
			if it.ID == id {
				return it
			}
		}
		t.Fatalf("no work item %q in the fixture", id)
		return api.WorkItem{}
	}

	lines := m.cardLines(byID("wi"), 40, false)
	if len(lines) != cardRows {
		t.Fatalf("a card with relations is %d rows, want cardRows=%d", len(lines), cardRows)
	}
	meta := lines[cardRows-1]
	// The parent badge is deliberately bare on a board card — no work item number, which is
	// one "g" away on the detail screen's own Parent: line instead (see cardRelations).
	if !strings.Contains(meta, "↑") {
		t.Errorf("card does not show that it has a parent:\n%q", meta)
	}
	if strings.Contains(meta, "↑100") {
		t.Errorf("board card should not show the parent's work item number:\n%q", meta)
	}
	if !strings.Contains(meta, "↳3") {
		t.Errorf("card does not show how many sub-tasks it has:\n%q", meta)
	}

	if meta := m.cardLines(byID("lone"), 40, false)[cardRows-1]; strings.ContainsAny(meta, "↑↳") {
		t.Errorf("an item with no parent and no sub-tasks got a relations badge:\n%q", meta)
	}

	// A parent the board has not loaded (archived, or a page still in flight) still has to
	// mark the card as a sub-task, just without a number to point at.
	orphan := api.WorkItem{ID: "orphan", SequenceID: 999, Name: "Child of an item we cannot see", State: "s1", Parent: "missing"}
	if rel := m.cardRelations(orphan); rel != "↑" {
		t.Errorf("cardRelations for an unknown parent = %q, want %q", rel, "↑")
	}
}

// TestDetailShowsParentAndSubTasksWithStateAndPriority covers the third part of the ask:
// related items are listed with their state and priority, not just their title.
func TestDetailShowsParentAndSubTasksWithStateAndPriority(t *testing.T) {
	m := relationsFixture(3)
	view := m.viewDetail()

	if !strings.Contains(view, "#100 The parent item") {
		t.Errorf("detail screen does not name the parent:\n%s", view)
	}
	if !strings.Contains(view, "Done") || !strings.Contains(view, "low") {
		t.Errorf("detail screen does not show the parent's state and priority:\n%s", view)
	}
	if !strings.Contains(view, "Sub-tasks: 3") {
		t.Errorf("detail screen does not count the sub-tasks:\n%s", view)
	}
	for _, want := range []string{"#300 Sub-task number 0", "#301 Sub-task number 1", "#302 Sub-task number 2"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail screen is missing sub-task %q:\n%s", want, view)
		}
	}
	if !strings.Contains(view, "In Progress") {
		t.Error("detail screen does not show a sub-task's state")
	}
}

// TestDetailMetaWithoutRelations checks a plain top-level work item says so, rather than
// leaving the two rows blank or dropping them.
func TestDetailMetaWithoutRelations(t *testing.T) {
	m := relationsFixture(0)
	lone := m.items[2]
	m.detailItem = &lone

	view := m.viewDetail()
	if !strings.Contains(view, "Parent:    —") {
		t.Errorf("a top-level item should say it has no parent:\n%s", view)
	}
	if !strings.Contains(view, "Sub-tasks: —") {
		t.Errorf("an item with no children should say so:\n%s", view)
	}
	if !strings.Contains(view, "Labels:    —") {
		t.Errorf("an item with no labels should say so:\n%s", view)
	}
}

// TestDetailMetaShowsLabels checks the meta block resolves label IDs to names, the same way
// a board card's meta line does (labelNames).
func TestDetailMetaShowsLabels(t *testing.T) {
	m := relationsFixture(0)
	lone := m.items[2]
	lone.Labels = []string{"l1", "l2"}
	m.detailItem = &lone
	m.labels = []api.Label{{ID: "l1", Name: "bug"}, {ID: "l2", Name: "urgent-fix"}}

	view := m.viewDetail()
	if !strings.Contains(view, "Labels:    bug, urgent-fix") {
		t.Errorf("detail screen does not show the item's labels:\n%s", view)
	}
}

// TestDetailInlineSubTaskListIsCapped checks the meta block does not grow without bound: a
// long child list is cut off with a count of what is left, which is what keeps the two panes
// from being squeezed out by a work item with thirty sub-tasks.
func TestDetailInlineSubTaskListIsCapped(t *testing.T) {
	m := relationsFixture(maxInlineSubTasks + 4)
	view := m.viewDetail()

	if !strings.Contains(view, fmt.Sprintf("… %d more", 4)) {
		t.Errorf("the inline sub-task list does not say how many it left out:\n%s", view)
	}
	last := fmt.Sprintf("#%d Sub-task number %d", 300+maxInlineSubTasks, maxInlineSubTasks)
	if strings.Contains(view, last) {
		t.Errorf("the inline sub-task list ran past maxInlineSubTasks (found %q)", last)
	}
}

// TestJumpToParent covers "navigate to a parent from the detail view": g opens the parent,
// on the same code path the board's enter takes, and is a no-op on a top-level item.
func TestJumpToParent(t *testing.T) {
	m := relationsFixture(2)

	next, cmd := m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = next.(Model)
	if m.detailItem == nil || m.detailItem.ID != "p" {
		t.Fatalf("g did not open the parent (detailItem=%+v)", m.detailItem)
	}
	if m.screen != screenDetail {
		t.Errorf("screen = %v, want the detail screen", m.screen)
	}
	if cmd == nil {
		t.Error("g did not refresh the item it jumped to in the background")
	}
	if !m.detailLoading || !m.commentsLoading || m.comments != nil {
		t.Error("jumping to the parent kept the previous item's comments on screen")
	}
	if m.focusedCol != 2 {
		t.Errorf("focusedCol = %d, want the parent's column (2) so esc lands on its card", m.focusedCol)
	}

	// The parent is top-level: g has nothing to do and must not move anywhere.
	next, _ = m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if got := next.(Model).detailItem; got == nil || got.ID != "p" {
		t.Errorf("g on a top-level item moved to %+v", got)
	}
}

// TestJumpToParentReportsAnUnreachableParent checks the one case the user cannot act on: a
// parent this client cannot see (archived, or its fetch failed) is reported rather than
// silently doing nothing.
func TestJumpToParentReportsAnUnreachableParent(t *testing.T) {
	m := relationsFixture(0)
	orphan := api.WorkItem{ID: "orphan", SequenceID: 999, Name: "Child of an archived item", State: "s1", Parent: "missing"}
	m.detailItem = &orphan

	next, cmd := m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = next.(Model)
	if cmd != nil {
		t.Error("g tried to open a parent the client does not have")
	}
	if m.err == "" {
		t.Error("g said nothing about a parent it could not open")
	}
	if m.detailItem.ID != "orphan" {
		t.Errorf("g moved off the work item on screen: %+v", m.detailItem)
	}
	if !strings.Contains(m.viewDetail(), "(not available)") {
		t.Error("the meta block does not mark an unreachable parent")
	}
}

// TestFetchMissingParentCachesIt covers the other half of an unloaded parent: opening the
// card asks the server for it, and once it lands the detail screen shows it like any other.
func TestFetchMissingParentCachesIt(t *testing.T) {
	m := relationsFixture(0)
	m.client = api.New("http://plane.test", "token")
	orphan := api.WorkItem{ID: "orphan", SequenceID: 999, Name: "Child of an unloaded item", State: "s1", Parent: "missing"}

	if cmd := m.fetchMissingParent(orphan); cmd == nil {
		t.Fatal("opening a card whose parent is not on the board did not fetch it")
	}
	if cmd := m.fetchMissingParent(m.items[1]); cmd != nil {
		t.Error("a parent already on the board was fetched again")
	}
	if cmd := m.fetchMissingParent(m.items[2]); cmd != nil {
		t.Error("a top-level item triggered a parent fetch")
	}

	fetched := api.WorkItem{ID: "missing", SequenceID: 42, Name: "The archived parent", State: "s3", Priority: "urgent"}
	next, _ := m.handleRelatedWorkItem(relatedWorkItemMsg{item: &fetched})
	m = next.(Model)
	m.detailItem = &orphan
	if !strings.Contains(m.viewDetail(), "#42 The archived parent") {
		t.Errorf("the fetched parent is not shown on the detail screen:\n%s", m.viewDetail())
	}

	// A failed fetch must not raise an error banner over whatever the user is doing.
	next, _ = m.handleRelatedWorkItem(relatedWorkItemMsg{err: fmt.Errorf("boom")})
	if next.(Model).err != "" {
		t.Errorf("a failed background parent fetch set an error banner: %q", next.(Model).err)
	}
}

// TestSubIssuePickerJumpsToTheSelectedChild covers "navigate to a sub-task": S lists them
// and enter opens the one under the cursor.
func TestSubIssuePickerJumpsToTheSelectedChild(t *testing.T) {
	m := relationsFixture(3)

	next, _ := m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	m = next.(Model)
	if m.pickerOpen != "subissue" {
		t.Fatalf("pickerOpen = %q, want the sub-task picker", m.pickerOpen)
	}
	if m.pickerOptionCount() != 3 {
		t.Fatalf("picker lists %d options, want 3", m.pickerOptionCount())
	}

	view := m.viewDetail()
	if !strings.Contains(view, "Jump to sub-task") || !strings.Contains(view, "#301 Sub-task number 1") {
		t.Errorf("the sub-task picker does not list the children with their details:\n%s", view)
	}

	next, _ = m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = next.(Model)
	next, cmd := m.updateDetail(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if m.pickerOpen != "" {
		t.Error("the picker stayed open after enter")
	}
	if m.detailItem == nil || m.detailItem.SequenceID != 301 {
		t.Fatalf("enter opened %+v, want sub-task #301", m.detailItem)
	}
	if cmd == nil {
		t.Error("enter did not refresh the sub-task it opened")
	}
	if m.status != "" {
		t.Errorf("status = %q — navigating must not look like a work item update", m.status)
	}
}

// TestSubIssuePickerOnAnItemWithNoChildren checks S says so rather than opening an empty
// picker with nothing to select.
func TestSubIssuePickerOnAnItemWithNoChildren(t *testing.T) {
	m := relationsFixture(0)

	next, _ := m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	m = next.(Model)
	if m.pickerOpen != "" {
		t.Errorf("pickerOpen = %q, want no picker for an item with no sub-tasks", m.pickerOpen)
	}
	if m.err == "" {
		t.Error("S said nothing on an item with no sub-tasks")
	}
}

// TestPickerStillUpdatesStateAndPriority guards the shared picker: adding a navigating
// picker to it must not stop the state/priority ones from PATCHing the work item.
func TestPickerStillUpdatesStateAndPriority(t *testing.T) {
	for _, tc := range []struct {
		key  rune
		want string
	}{{'s', "state"}, {'y', "priority"}} {
		m := relationsFixture(2)
		m.client = api.New("http://plane.test", "token")

		next, _ := m.updateDetail(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
		m = next.(Model)
		if m.pickerOpen != tc.want {
			t.Fatalf("%c opened %q, want %q", tc.key, m.pickerOpen, tc.want)
		}
		next, cmd := m.updateDetail(tea.KeyMsg{Type: tea.KeyEnter})
		m = next.(Model)
		if cmd == nil {
			t.Errorf("%c picker did not save anything", tc.key)
		}
		if m.status == "" {
			t.Errorf("%c picker did not report that it was updating", tc.key)
		}
	}
}

// TestFocusBoardOnFollowsTheItemJumpedTo checks backing out of a work item reached through a
// relation lands on its card rather than wherever the user started, and that a card the
// board is not showing leaves the cursor alone instead of pointing at the wrong one.
func TestFocusBoardOnFollowsTheItemJumpedTo(t *testing.T) {
	m := relationsFixture(3)
	m.focusedCol, m.colCursor = 0, []int{0, 0, 0}

	sub := m.subIssues("wi")[2] // #302, state s3 ("Done"), the fixture's third column
	next, _ := m.openWorkItem(sub)
	m = next.(Model)
	if m.focusedCol != 2 {
		t.Fatalf("focusedCol = %d, want 2 (the sub-task's column)", m.focusedCol)
	}
	col := m.columnItems("s3")
	if idx := m.colCursor[2]; idx >= len(col) || col[idx].ID != sub.ID {
		t.Errorf("colCursor[2] = %d, which is not the card that was opened", m.colCursor[2])
	}

	// Filtered off the board: there is no card to land on, so the cursor must not move.
	m.filterLabel = "no-such-label"
	m.focusedCol, m.colCursor = 1, []int{0, 5, 0}
	next, _ = m.openWorkItem(sub)
	m = next.(Model)
	if m.focusedCol != 1 || m.colCursor[1] != 5 {
		t.Errorf("opening a filtered-out item moved the board cursor to col %d idx %d", m.focusedCol, m.colCursor[1])
	}
}

func TestPickerWindow(t *testing.T) {
	for _, tc := range []struct {
		n, idx, max          int
		wantStart, wantCount int
		wantScrolled         bool
	}{
		{3, 0, 8, 0, 3, false},  // everything fits
		{8, 7, 8, 0, 8, false},  // exactly fits
		{20, 0, 8, 0, 8, true},  // at the top
		{20, 10, 8, 6, 8, true}, // centred on the selection
		{20, 19, 8, 12, 8, true},
	} {
		start, count, scrolled := pickerWindow(tc.n, tc.idx, tc.max)
		if start != tc.wantStart || count != tc.wantCount || scrolled != tc.wantScrolled {
			t.Errorf("pickerWindow(%d,%d,%d) = (%d,%d,%v), want (%d,%d,%v)",
				tc.n, tc.idx, tc.max, start, count, scrolled, tc.wantStart, tc.wantCount, tc.wantScrolled)
		}
		if start < 0 || start+count > tc.n || (tc.idx < tc.n && (tc.idx < start || tc.idx >= start+count)) {
			t.Errorf("pickerWindow(%d,%d,%d) = (%d,%d): window is out of range or misses the selection",
				tc.n, tc.idx, tc.max, start, count)
		}
	}
}

// TestViewDetailFitsTerminalWithSubIssuePicker is TestViewDetailFitsTerminalWithOverlays for
// the new picker, whose option count comes from project data rather than a fixed list: a work
// item with thirty sub-tasks must still render a frame that fits the terminal, hint row and
// all, on every size down to the classic 80x24.
func TestViewDetailFitsTerminalWithSubIssuePicker(t *testing.T) {
	sizes := []struct{ width, height int }{
		{230, 52}, {200, 50}, {160, 48}, {120, 40}, {100, 30}, {80, 24},
	}
	for _, size := range sizes {
		m := relationsFixture(30)
		m.width, m.height = size.width, size.height
		m.openSubIssuePicker()

		view := m.viewDetail()
		rows, cols := frameSize(view)
		if rows > size.height {
			t.Errorf("%dx%d: frame is %d rows tall, want <= %d", size.width, size.height, rows, size.height)
		}
		if cols > size.width {
			t.Errorf("%dx%d: frame is %d columns wide, want <= %d", size.width, size.height, cols, size.width)
		}
		// The picker is the interactive part of the frame: clipping its last rows would take
		// the selected option or the way out of it off the screen.
		if !strings.Contains(view, "esc  cancel") {
			t.Errorf("%dx%d: the sub-task picker's hint row was clipped off the frame:\n%s", size.width, size.height, view)
		}
	}
}

// TestViewBoardFitsTerminalWithRelations is TestViewBoardFitsTerminal for a board where every
// card carries a relations badge — the badge shares the card's existing meta line, so this is
// the check that it did not quietly cost the board a row.
func TestViewBoardFitsTerminalWithRelations(t *testing.T) {
	sizes := []struct{ width, height int }{
		{230, 52}, {120, 40}, {80, 24}, {60, 20},
	}
	for _, size := range sizes {
		m := boardFixture(5, 60, size.width, size.height)
		m.cfg = config.Config{}
		// Chain every card onto the one before it, so each has both a parent and a child.
		for i := 1; i < len(m.items); i++ {
			m.items[i].Parent = m.items[i-1].ID
		}
		rows, cols := frameSize(m.viewBoard())
		if rows > size.height {
			t.Errorf("%dx%d: frame is %d rows tall, want <= %d", size.width, size.height, rows, size.height)
		}
		if cols > size.width {
			t.Errorf("%dx%d: frame is %d columns wide, want <= %d", size.width, size.height, cols, size.width)
		}
	}
}
