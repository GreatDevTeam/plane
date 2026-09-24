package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// TestColumnItemsFiltersByTitleSearch covers the board's title search (m.titleSearch, set by
// "/" — see updateTitleSearch): a plain case-insensitive substring match against already-loaded
// items, composing with the existing assignee/label/state/priority filters.
func TestColumnItemsFiltersByTitleSearch(t *testing.T) {
	m := Model{
		states: []api.State{{ID: "s1"}},
		items: []api.WorkItem{
			{ID: "1", State: "s1", Name: "Fix the login bug", Assignees: []string{"u1"}},
			{ID: "2", State: "s1", Name: "Add dark mode"},
			{ID: "3", State: "s1", Name: "Refactor LOGIN flow", Assignees: []string{"u1"}},
		},
	}

	if got := len(m.columnItems("s1")); got != 3 {
		t.Fatalf("no search: got %d items, want 3", got)
	}

	m.titleSearch = "login"
	if got := ids2(m.columnItems("s1")); len(got) != 2 || got[0] != "1" || got[1] != "3" {
		t.Fatalf("title search = %v, want [1 3] (case-insensitive substring match)", got)
	}

	m.filterAssignee = "u1"
	if got := len(m.columnItems("s1")); got != 2 {
		t.Fatalf("title search + assignee filter: got %d items, want 2", got)
	}

	m.titleSearch = "nothing matches this"
	if got := len(m.columnItems("s1")); got != 0 {
		t.Fatalf("non-matching search: got %d items, want 0", got)
	}
}

func ids2(items []api.WorkItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

// TestUpdateTitleSearchOpenTypeEnterEsc drives the "/" title search the way a user does: open
// it, type to live-filter the board, enter keeps the query applied and returns to board
// navigation, esc (re-opened) clears it back to showing everything.
func TestUpdateTitleSearchOpenTypeEnterEsc(t *testing.T) {
	m := boardFixture(1, 2, 120, 40)
	m.items = []api.WorkItem{
		{ID: "1", State: "s0", Name: "Fix the login bug"},
		{ID: "2", State: "s0", Name: "Add dark mode"},
	}
	m.colCursor = []int{1}

	next, _ := m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = next.(Model)
	if !m.titleSearchOpen {
		t.Fatal("/ did not open the title search box")
	}

	for _, r := range "login" {
		next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	if m.titleSearch != "login" {
		t.Fatalf("titleSearch = %q, want %q to update live while typing", m.titleSearch, "login")
	}
	if got := len(m.columnItems("s0")); got != 1 {
		t.Fatalf("live filter: got %d items, want 1", got)
	}
	if m.colCursor[0] != 0 {
		t.Fatalf("colCursor[0] = %d, want 0 after the list narrowed under it", m.colCursor[0])
	}

	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.titleSearchOpen {
		t.Fatal("enter left the title search box open")
	}
	if m.titleSearch != "login" {
		t.Fatalf("titleSearch = %q after enter, want it kept applied", m.titleSearch)
	}

	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = next.(Model)
	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.titleSearchOpen || m.titleSearch != "" {
		t.Fatalf("esc did not clear the search: open=%v query=%q", m.titleSearchOpen, m.titleSearch)
	}
	if got := len(m.columnItems("s0")); got != 2 {
		t.Fatalf("after clearing: got %d items, want 2", got)
	}
}

// TestLabelFilterSearchNarrowsAndApplies covers "L" (filter by label): typing narrows the
// picker's option list (filterOptionLabels/filterOptionIDs), and enter applies whichever
// filtered row is highlighted.
func TestLabelFilterSearchNarrowsAndApplies(t *testing.T) {
	m := boardFixture(1, 1, 120, 40)
	m.labels = []api.Label{
		{ID: "l1", Name: "bug"},
		{ID: "l2", Name: "feature"},
		{ID: "l3", Name: "bugfix-candidate"},
	}

	next, _ := m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'L'}})
	m = next.(Model)
	if m.filterOpen != "label" {
		t.Fatalf("L did not open the label filter, filterOpen = %q", m.filterOpen)
	}
	if got := len(m.filterOptionLabels()); got != 3 {
		t.Fatalf("no query: got %d options, want 3", got)
	}

	for _, r := range "bug" {
		next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	opts := m.filterOptionLabels()
	if len(opts) != 2 {
		t.Fatalf("query %q: got %d options, want 2 (bug, bugfix-candidate): %v", "bug", len(opts), opts)
	}

	// The first filtered row ("All" is index 0, so index 1 is the first match) is "bug".
	m.filterIdx = 1
	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.filterOpen != "" {
		t.Fatal("enter did not close the label filter picker")
	}
	if m.filterLabel != "l1" {
		t.Fatalf("filterLabel = %q, want l1 (bug)", m.filterLabel)
	}
}

// TestHandleWorkItemCreatedFocusesNewItemUnderSortMode covers the board's "n" create flow when
// a non-default sort mode is active: the new item can land anywhere in the sorted column, not
// necessarily last, so the cursor must follow its actual position rather than assuming the end.
func TestHandleWorkItemCreatedFocusesNewItemUnderSortMode(t *testing.T) {
	m := boardFixture(1, 0, 120, 40)
	m.editor = newTestEditor()
	m.items = []api.WorkItem{
		{ID: "1", State: "s0", SequenceID: 1, Priority: "low"},
		{ID: "2", State: "s0", SequenceID: 2, Priority: "medium"},
	}
	m.colCursor = []int{0}
	m.sortMode = "priority"

	created := api.WorkItem{ID: "new-1", SequenceID: 3, State: "s0", Priority: "urgent"}
	next, _ := m.handleWorkItemCreated(workItemCreatedMsg{item: &created})
	m = next.(Model)

	col := m.columnItems("s0")
	wantIdx := -1
	for i, it := range col {
		if it.ID == "new-1" {
			wantIdx = i
		}
	}
	if wantIdx != 0 {
		t.Fatalf("urgent item sorted to index %d, want 0 (urgent sorts first)", wantIdx)
	}
	if m.colCursor[0] != wantIdx {
		t.Fatalf("colCursor[0] = %d, want %d (the new item's actual sorted position)", m.colCursor[0], wantIdx)
	}
}
