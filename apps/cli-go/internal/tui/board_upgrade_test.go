package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
	"github.com/makeplane/plane/apps/cli-go/internal/config"
)

// hideEvery collapses every n-th column of a board fixture.
func hideEvery(m *Model, n int) {
	m.hiddenStates = make(map[string]bool)
	for i, st := range m.states {
		if i%n == 0 {
			m.hiddenStates[st.ID] = true
		}
	}
}

// TestViewBoardFitsTerminalWithHiddenColumns is TestViewBoardFitsTerminal for a board with
// collapsed columns: a placeholder column is a different width from every other column, so
// the layout arithmetic has to account for it or the frame overflows the terminal again.
func TestViewBoardFitsTerminalWithHiddenColumns(t *testing.T) {
	sizes := []struct{ width, height int }{
		{230, 52}, {160, 48}, {120, 40}, {100, 30}, {80, 24}, {60, 20}, {40, 12}, {24, 8},
	}
	for _, columns := range []int{2, 5, 9, 14} {
		for _, every := range []int{1, 2, 3} {
			for _, size := range sizes {
				m := boardFixture(columns, 900, size.width, size.height)
				m.focusedCol = columns / 2
				hideEvery(&m, every)
				rows, cols := frameSize(m.viewBoard())
				if rows > size.height {
					t.Errorf("%d columns (every %d hidden) on a %dx%d terminal: frame is %d rows tall",
						columns, every, size.width, size.height, rows)
				}
				if cols > size.width {
					t.Errorf("%d columns (every %d hidden) on a %dx%d terminal: frame is %d columns wide",
						columns, every, size.width, size.height, cols)
				}
			}
		}
	}
}

// TestBoardLayoutForFitsWidthWithHiddenColumns pins the layout arithmetic itself: whatever
// mix of collapsed and expanded columns a board has, the window it picks must fit the
// terminal and must contain the focused column.
func TestBoardLayoutForFitsWidthWithHiddenColumns(t *testing.T) {
	for width := 20; width <= 260; width += 3 {
		for _, columns := range []int{1, 2, 3, 6, 12} {
			for _, every := range []int{1, 2, 3} {
				hidden := make([]bool, columns)
				for i := range hidden {
					hidden[i] = i%every == 0
				}
				for _, focused := range []int{0, columns / 2, columns - 1} {
					colWidth, first, visible := boardLayoutFor(width, hidden, focused)
					if visible < 1 || visible > columns {
						t.Fatalf("width=%d columns=%d: showed %d columns", width, columns, visible)
					}
					if focused < first || focused >= first+visible {
						t.Fatalf("width=%d columns=%d focused=%d: window [%d,%d) leaves the focused column off screen",
							width, columns, focused, first, first+visible)
					}
					used := 0
					for i := first; i < first+visible; i++ {
						if hidden[i] {
							used += hiddenColWidth + colFrame
							continue
						}
						if colWidth < minColWidth {
							t.Fatalf("width=%d columns=%d: column width %d is below minColWidth %d",
								width, columns, colWidth, minColWidth)
						}
						used += colWidth + colFrame
					}
					if used > width && width >= minColWidth+colFrame {
						t.Fatalf("width=%d columns=%d: the chosen window needs %d terminal columns", width, columns, used)
					}
				}
			}
		}
	}
}

// TestHiddenColumnGivesItsWidthToTheOthers is the point of collapsing a column: the board
// gets to show more of the columns the user still cares about.
func TestHiddenColumnGivesItsWidthToTheOthers(t *testing.T) {
	const width = 100
	hidden := make([]bool, 6)
	_, _, visibleBefore := boardLayoutFor(width, hidden, 0)
	for i := range hidden {
		hidden[i] = i >= 3 // collapse the last three
	}
	_, _, visibleAfter := boardLayoutFor(width, hidden, 0)
	if visibleAfter <= visibleBefore {
		t.Errorf("collapsing 3 of 6 columns showed %d columns, want more than the %d shown before",
			visibleAfter, visibleBefore)
	}
}

// TestRenderCollapsedColumnSize keeps a placeholder inside the box the layout budgeted for
// it: hiddenColWidth+colFrame terminal columns, and never taller than a full column.
func TestRenderCollapsedColumnSize(t *testing.T) {
	for _, maxRows := range []int{3, 10, 40} {
		m := boardFixture(3, 90, 120, 40)
		m.hiddenStates = map[string]bool{"s1": true}
		m.states[1].Name = "In Progress With A Very Long Name"
		out := m.renderCollapsedColumn(1, maxRows)
		if got, want := lipgloss.Width(out), hiddenColWidth+colFrame; got != want {
			t.Errorf("maxRows=%d: placeholder is %d columns wide, want %d", maxRows, got, want)
		}
		if got, want := lipgloss.Height(out), maxRows*cardRows+colChromeRows; got > want {
			t.Errorf("maxRows=%d: placeholder is %d rows tall, want <= %d", maxRows, got, want)
		}
	}
}

// TestHiddenColumnShowsNoCards checks a collapsed column really is collapsed: its cards are
// gone from the frame, and nothing in it is selected (so s/y/enter cannot act on a card the
// user cannot see).
func TestHiddenColumnShowsNoCards(t *testing.T) {
	m := boardFixture(3, 9, 200, 40)
	m.states[0].Name = "Backlog"
	m.items[0].Name = "a card only in the first column"
	m.items[0].State = m.states[0].ID

	if !strings.Contains(m.viewBoard(), "a card only in the first") {
		t.Fatal("card is not on the board before its column is hidden")
	}
	m.hiddenStates = map[string]bool{m.states[0].ID: true}
	if strings.Contains(m.viewBoard(), "a card only in the first") {
		t.Error("a hidden column still rendered its cards")
	}
	m.focusedCol = 0
	if m.selectedItem() != nil {
		t.Error("a hidden column still has a selected item")
	}
}

// TestToggleHiddenColumnPersists checks x both collapses the focused column and remembers it
// for the next run, per project.
func TestToggleHiddenColumnPersists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := boardFixture(3, 9, 120, 40)
	m.project = api.Project{ID: "proj-1", Name: "Proj"}
	m.focusedCol = 1

	next, _ := m.toggleHiddenColumn()
	m = next.(Model)
	if !m.isHidden(m.states[1].ID) {
		t.Fatal("x did not hide the focused column")
	}

	saved, err := config.Load()
	if err != nil {
		t.Fatalf("loading the saved config: %v", err)
	}
	if !saved.HiddenStatesFor("proj-1")[m.states[1].ID] {
		t.Errorf("hidden column was not persisted: %#v", saved.HiddenStates)
	}
	if len(saved.HiddenStatesFor("other-project")) != 0 {
		t.Error("hiding a column leaked into another project")
	}

	next, _ = m.toggleHiddenColumn()
	m = next.(Model)
	if m.isHidden(m.states[1].ID) {
		t.Fatal("x did not bring the column back")
	}
	saved, _ = config.Load()
	if len(saved.HiddenStatesFor("proj-1")) != 0 {
		t.Errorf("un-hiding a column was not persisted: %#v", saved.HiddenStates)
	}
}

// TestSortWorkItems covers every card ordering the board offers.
func TestSortWorkItems(t *testing.T) {
	items := []api.WorkItem{
		{SequenceID: 3, Name: "beta", Priority: "low", CreatedAt: "2026-01-03T00:00:00Z", UpdatedAt: "2026-02-01T00:00:00Z"},
		{SequenceID: 1, Name: "Alpha", Priority: "urgent", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-09T00:00:00Z"},
		{SequenceID: 2, Name: "gamma", Priority: "", CreatedAt: "2026-01-02T00:00:00Z", UpdatedAt: "2026-03-01T00:00:00Z"},
	}
	cases := []struct {
		mode string
		want []int // expected sequence IDs, in order
	}{
		{sortDefault, []int{3, 1, 2}},
		{"priority", []int{1, 3, 2}},
		{"created-desc", []int{3, 2, 1}},
		{"created-asc", []int{1, 2, 3}},
		{"updated-desc", []int{2, 3, 1}},
		{"name", []int{1, 3, 2}},
		{"id-asc", []int{1, 2, 3}},
		{"nonsense-from-an-edited-config", []int{3, 1, 2}},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			got := append([]api.WorkItem(nil), items...)
			sortWorkItems(got, tc.mode)
			for i, want := range tc.want {
				if got[i].SequenceID != want {
					t.Fatalf("order = %v, want %v", ids(got), tc.want)
				}
			}
		})
	}
}

func ids(items []api.WorkItem) []int {
	out := make([]int, len(items))
	for i, it := range items {
		out[i] = it.SequenceID
	}
	return out
}

// TestColumnItemsAppliesSortMode checks the ordering reaches the board (and still composes
// with the filters).
func TestColumnItemsAppliesSortMode(t *testing.T) {
	m := Model{
		states: []api.State{{ID: "s1"}},
		items: []api.WorkItem{
			{ID: "1", State: "s1", SequenceID: 7, Priority: "low", Assignees: []string{"u1"}},
			{ID: "2", State: "s1", SequenceID: 5, Priority: "urgent"},
			{ID: "3", State: "s1", SequenceID: 6, Priority: "high", Assignees: []string{"u1"}},
		},
		sortMode: "priority",
	}
	if got := ids(m.columnItems("s1")); got[0] != 5 || got[1] != 6 || got[2] != 7 {
		t.Errorf("priority order = %v, want [5 6 7]", got)
	}
	m.filterAssignee = "u1"
	if got := ids(m.columnItems("s1")); len(got) != 2 || got[0] != 6 || got[1] != 7 {
		t.Errorf("filtered priority order = %v, want [6 7]", got)
	}
}

// TestSortPickerAppliesAndPersists drives the o picker the way a user does.
func TestSortPickerAppliesAndPersists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := boardFixture(3, 9, 120, 40)
	m.openSortPicker()
	if m.pickerOpen != "sort" || m.pickerIdx != 0 {
		t.Fatalf("openSortPicker: pickerOpen=%q pickerIdx=%d, want sort/0", m.pickerOpen, m.pickerIdx)
	}
	next, _ := m.updatePicker(tea.KeyMsg{Type: tea.KeyDown})
	next, _ = next.(Model).updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if m.pickerOpen != "" {
		t.Error("the picker stayed open after enter")
	}
	if m.sortMode != sortModes[1].key {
		t.Errorf("sortMode = %q, want %q", m.sortMode, sortModes[1].key)
	}
	saved, err := config.Load()
	if err != nil {
		t.Fatalf("loading the saved config: %v", err)
	}
	if saved.SortMode != sortModes[1].key {
		t.Errorf("persisted sort mode = %q, want %q", saved.SortMode, sortModes[1].key)
	}
	if !strings.Contains(m.viewBoard(), "sorted by") {
		t.Error("the board header does not say which order it is in")
	}
}

// TestApplyBoardRefreshKeepsThePlaceTheUserWasIn is what makes the 30s refresh invisible: the
// board is swapped out whole, but the focused column, every column's cursor and the active
// filters must survive it — including when a state was added in front of the others.
func TestApplyBoardRefreshKeepsThePlaceTheUserWasIn(t *testing.T) {
	m := boardFixture(3, 30, 120, 40)
	for i := range m.items {
		m.items[i].Labels = []string{"l1"}
	}
	m.focusedCol = 2
	m.colCursor = []int{1, 2, 3}
	m.filterLabel = "l1"

	states := append([]api.State{{ID: "new", Name: "Triage"}}, m.states...)
	items := append([]api.WorkItem(nil), m.items...)
	m.applyBoardRefresh(states, items, nil)

	if got := m.states[m.focusedCol].ID; got != "s2" {
		t.Errorf("focused column is now %q, want the state the user was on (s2)", got)
	}
	if len(m.colCursor) != len(states) {
		t.Fatalf("colCursor has %d entries for %d states", len(m.colCursor), len(states))
	}
	if m.colCursor[m.focusedCol] != 3 {
		t.Errorf("cursor of the focused column = %d, want 3", m.colCursor[m.focusedCol])
	}
	if m.filterLabel != "l1" {
		t.Error("the active filter was dropped by a refresh")
	}
}

// TestApplyBoardRefreshClampsCursorsOntoShorterColumns covers the other direction: cards can
// disappear between two refreshes, and the cursor must not point past the end of a column.
func TestApplyBoardRefreshClampsCursorsOntoShorterColumns(t *testing.T) {
	m := boardFixture(2, 20, 120, 40)
	m.focusedCol = 1
	m.colCursor = []int{5, 5}

	m.applyBoardRefresh(m.states, []api.WorkItem{{ID: "only", State: "s1", Name: "last one"}}, nil)

	for i, c := range m.colCursor {
		if c != 0 {
			t.Errorf("cursor of column %d = %d, want 0 after every other card disappeared", i, c)
		}
	}
	if rows, _ := frameSize(m.viewBoard()); rows > 40 {
		t.Errorf("frame is %d rows tall after a refresh", rows)
	}
}

// TestBoardTickKeepsTicking checks the 30s refresh survives a tick it had to skip: the timer
// is re-armed whether or not the tick started a refresh.
func TestBoardTickKeepsTicking(t *testing.T) {
	m := boardFixture(3, 9, 120, 40)
	m.screen = screenBoard
	m.client = api.New("http://example.invalid", "token")
	m.project = api.Project{ID: "proj-1"}

	if _, cmd := m.handleBoardTick(boardTickMsg{}); cmd == nil {
		t.Fatal("a tick that refreshes did not re-arm the timer")
	}
	m.editorOn = true // the user is typing: this tick must be skipped
	if !m.shouldSkipRefresh() {
		t.Error("a refresh while an editor is open must be skipped")
	}
	if _, cmd := m.handleBoardTick(boardTickMsg{}); cmd == nil {
		t.Fatal("a skipped tick did not re-arm the timer")
	}
}

// TestShouldSkipRefresh pins the rest of the guard: nothing that is mid-flight or mid-choice
// gets pulled out from under the user by a background refresh.
func TestShouldSkipRefresh(t *testing.T) {
	base := func() Model {
		m := boardFixture(3, 9, 120, 40)
		m.screen = screenBoard
		m.client = api.New("http://example.invalid", "token")
		m.project = api.Project{ID: "proj-1"}
		return m
	}
	if base().shouldSkipRefresh() {
		t.Fatal("an idle board must refresh")
	}
	cases := map[string]func(*Model){
		"initial load":           func(m *Model) { m.loading = true },
		"already refreshing":     func(m *Model) { m.refreshing = true },
		"paging in items":        func(m *Model) { m.itemsLoadingMore = true },
		"picker open":            func(m *Model) { m.pickerOpen = "state" },
		"filter open":            func(m *Model) { m.filterOpen = "label" },
		"id prompt open":         func(m *Model) { m.idPromptOpen = true },
		"on the projects screen": func(m *Model) { m.screen = screenProjects },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			m := base()
			setup(&m)
			if !m.shouldSkipRefresh() {
				t.Errorf("a refresh during %q must be skipped", name)
			}
		})
	}
	t.Run("detail screen", func(t *testing.T) {
		m := base()
		m.screen = screenDetail
		if m.shouldSkipRefresh() {
			t.Error("the board must keep refreshing while a card is open")
		}
	})
}

// TestBoardRefreshDoesNotClearTheBoard is the "must not blink" requirement: a refresh never
// goes through the loading screen, so there is no frame in which the board is empty.
func TestBoardRefreshDoesNotClearTheBoard(t *testing.T) {
	m := boardFixture(3, 30, 120, 40)
	m.screen = screenBoard
	m.client = api.New("http://example.invalid", "token")
	m.project = api.Project{ID: "proj-1"}

	next, _ := m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(Model)
	if m.loading {
		t.Error("refreshing put the board back on the loading screen")
	}
	if len(m.items) == 0 {
		t.Error("refreshing emptied the board")
	}
	if !strings.Contains(m.viewBoard(), longNames[0][:20]) {
		t.Error("the board stopped showing its cards while refreshing")
	}
	if !strings.Contains(m.viewBoard(), "refreshing") {
		t.Error("nothing on screen says a refresh is in flight")
	}
}

// TestBoardStatesRenderInTheOrderGiven guards the column order the API client now sorts:
// viewBoard must not reorder the states underneath it.
func TestBoardStatesRenderInTheOrderGiven(t *testing.T) {
	m := boardFixture(3, 9, 200, 40)
	for i := range m.states {
		m.states[i].Name = fmt.Sprintf("Col%d", i)
	}
	view := m.viewBoard()
	first, second, third := strings.Index(view, "Col0"), strings.Index(view, "Col1"), strings.Index(view, "Col2")
	if first < 0 || second < first || third < second {
		t.Errorf("columns are not rendered left to right in state order (%d, %d, %d)", first, second, third)
	}
}
