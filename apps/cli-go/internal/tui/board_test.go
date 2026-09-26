package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
	"github.com/muesli/termenv"
)

func TestColumnItemsFiltersByAssigneeAndLabel(t *testing.T) {
	m := Model{
		states: []api.State{{ID: "s1"}},
		items: []api.WorkItem{
			{ID: "1", State: "s1", Assignees: []string{"u1"}, Labels: []string{"l1"}, Priority: "high"},
			{ID: "2", State: "s1", Assignees: []string{"u2"}, Labels: []string{"l1"}, Priority: "low"},
			{ID: "3", State: "s1", Assignees: []string{"u1"}, Labels: []string{"l2"}, Priority: "high"},
		},
	}

	if got := len(m.columnItems("s1")); got != 3 {
		t.Fatalf("no filter: got %d items, want 3", got)
	}

	m.filterAssignee = "u1"
	if got := len(m.columnItems("s1")); got != 2 {
		t.Fatalf("assignee filter: got %d items, want 2", got)
	}

	m.filterLabel = "l1"
	if got := len(m.columnItems("s1")); got != 1 {
		t.Fatalf("assignee+label filter: got %d items, want 1", got)
	}

	m.filterAssignee = ""
	m.filterLabel = ""
	m.filterPriority = "high"
	if got := len(m.columnItems("s1")); got != 2 {
		t.Fatalf("priority filter: got %d items, want 2", got)
	}

	m.filterPriority = ""
	m.filterState = "s1"
	if got := len(m.columnItems("s1")); got != 3 {
		t.Fatalf("state filter matching the column: got %d items, want 3", got)
	}

	m.filterState = "s2"
	if got := len(m.columnItems("s1")); got != 0 {
		t.Fatalf("state filter for a different state: got %d items, want 0", got)
	}
}

// longNames are deliberately awkward work item titles: longer than any column is ever going to
// be, and a mix of ASCII and Cyrillic so byte-length and display-width disagree.
var longNames = []string{
	"Port the company_slug fix to truckloads of downstream services",
	"Bound crawler impact on the lovable frontend before the next release",
	"k8s: перевірити CFS-throttling на воркерах у проді",
	"Прибрати публічний доступ до адмінки",
	"short",
}

// boardFixture builds a board with `items` work items spread over `columns` states, using the
// awkward titles above — i.e. the "big lists of tasks" the report was filed against.
func boardFixture(columns, items, width, height int) Model {
	states := make([]api.State, columns)
	for i := range states {
		states[i] = api.State{ID: fmt.Sprintf("s%d", i), Name: fmt.Sprintf("State %d", i)}
	}
	work := make([]api.WorkItem, items)
	for i := range work {
		work[i] = api.WorkItem{
			ID:         fmt.Sprintf("id-%d", i),
			SequenceID: 1000 + i,
			Name:       longNames[i%len(longNames)],
			State:      states[i%columns].ID,
		}
	}
	return Model{
		project:          api.Project{Name: "A Project With A Fairly Long Name"},
		states:           states,
		items:            work,
		colCursor:        make([]int, columns),
		width:            width,
		height:           height,
		pickerSearch:     newInput("", 40),
		filterSearch:     newInput("", 40),
		titleSearchInput: newInput("", 40),
		colorInput:       newInput("", 10),
		attachPathInput:  newInput("", 60),
	}
}

// TestOpenNewItemEditorAndCreate covers the whole "n" create-from-board flow: opening the
// editor targets the focused column's state, enter on the title moves to the review step
// (state/priority/assignee, still changeable there), and enter on that step fires the create
// and, once it comes back, appends the item to m.items and moves that column's cursor onto
// it — the same local-state update handleCommentSaved does after posting a comment.
func TestOpenNewItemEditorAndCreate(t *testing.T) {
	m := boardFixture(2, 2, 120, 40)
	m.editor = newTestEditor()
	m.focusedCol = 1

	next, _ := m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = next.(Model)
	if !m.editorOn || m.editorMode != "new-item" {
		t.Fatalf("n did not open the new-item editor (on=%v mode=%q)", m.editorOn, m.editorMode)
	}
	if m.newItemStateID != "s1" {
		t.Fatalf("newItemStateID = %q, want the focused column's state s1", m.newItemStateID)
	}

	m.editor.SetValue("A new card")
	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.editorOn {
		t.Fatal("enter on the title left the editor open instead of moving to the review step")
	}
	if !m.creatingItem || m.newItemName != "A new card" {
		t.Fatalf("creatingItem=%v newItemName=%q, want the review step with the typed title", m.creatingItem, m.newItemName)
	}

	next, cmd := m.updateBoard(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("enter on the review step did not save the new work item")
	}
	if !strings.Contains(m.status, "Creating work item") {
		t.Errorf("status = %q, want it to mention creating the work item", m.status)
	}

	created := api.WorkItem{ID: "new-1", SequenceID: 9999, Name: "A new card", State: "s1"}
	next, _ = m.handleWorkItemCreated(workItemCreatedMsg{item: &created})
	m = next.(Model)
	if m.editorOn {
		t.Error("the editor stayed open after the create came back")
	}
	if m.creatingItem {
		t.Error("creatingItem stayed true after the create came back")
	}
	if len(m.items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(m.items))
	}
	col := m.columnItems("s1")
	if len(col) == 0 || col[len(col)-1].ID != "new-1" {
		t.Fatalf("new item not appended to column s1: %+v", col)
	}
	if m.focusedCol != 1 || m.colCursor[1] != len(col)-1 {
		t.Errorf("cursor = col %d idx %d, want col 1 idx %d", m.focusedCol, m.colCursor[1], len(col)-1)
	}
}

// TestNewItemReviewPickersChangeStatePriorityAssignee covers the review step's s/y/a/T pickers:
// each should land its choice in m.newItem*, not PATCH anything (there is no work item yet),
// and the create should then send exactly what was picked.
func TestNewItemReviewPickersChangeStatePriorityAssignee(t *testing.T) {
	m := boardFixture(2, 2, 120, 40)
	m.editor = newTestEditor()
	m.members = []api.Member{{ID: "u1", DisplayName: "jane"}, {ID: "u2", DisplayName: "joe"}}
	m.focusedCol = 0

	next, _ := m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = next.(Model)
	m.editor.SetValue("Reviewed card")
	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.creatingItem {
		t.Fatal("expected the review step after confirming the title")
	}

	// s -> pick state index 1 ("s1").
	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = next.(Model)
	if m.pickerOpen != "state" {
		t.Fatalf("pickerOpen = %q, want state", m.pickerOpen)
	}
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.pickerOpen != "" || m.newItemStateID != "s1" {
		t.Fatalf("newItemStateID = %q (pickerOpen=%q), want s1 and picker closed", m.newItemStateID, m.pickerOpen)
	}

	// y -> the picker opens on the current "none" (the last entry); one "up" moves to "low".
	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = next.(Model)
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(Model)
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.newItemPriority != "low" {
		t.Fatalf("newItemPriority = %q, want low", m.newItemPriority)
	}

	// a -> the picker opens on "Unassigned" (idx 0); two "down"s reach member u2 (idx 2, past
	// the leading "Unassigned" entry and u1).
	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = next.(Model)
	if m.pickerOpen != "new-item-assignee" {
		t.Fatalf("pickerOpen = %q, want new-item-assignee", m.pickerOpen)
	}
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.newItemAssignee != "u2" {
		t.Fatalf("newItemAssignee = %q, want u2", m.newItemAssignee)
	}

	// T -> toggle the first label on with space, then save with enter.
	m.labels = []api.Label{{ID: "l1", Name: "bug"}, {ID: "l2", Name: "docs"}}
	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	m = next.(Model)
	if m.pickerOpen != "labels" {
		t.Fatalf("pickerOpen = %q, want labels", m.pickerOpen)
	}
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(Model)
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if len(m.newItemLabels) != 1 || m.newItemLabels[0] != "l1" {
		t.Fatalf("newItemLabels = %v, want [l1]", m.newItemLabels)
	}

	// No API calls should have happened yet — nothing to update, the item does not exist.
	_, cmd := m.updateBoard(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on the review step should fire the create")
	}
}

// TestNewItemReviewDescriptionEditor covers the review step's "d" key: it opens the shared
// editor in "new-item-description" mode, and ctrl+s stores the typed text as HTML in
// m.newItemDescription — a PATCH-free save, since the item does not exist yet — and returns to
// the review step with creatingItem still true.
func TestNewItemReviewDescriptionEditor(t *testing.T) {
	m := boardFixture(2, 2, 120, 40)
	m.editor = newTestEditor()

	next, _ := m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = next.(Model)
	m.editor.SetValue("Card with a description")
	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = next.(Model)
	if !m.editorOn || m.editorMode != "new-item-description" {
		t.Fatalf("d did not open the new-item description editor (on=%v mode=%q)", m.editorOn, m.editorMode)
	}

	m.editor.SetValue("Steps to reproduce")
	next, cmd := m.updateBoard(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = next.(Model)
	if cmd != nil {
		t.Error("ctrl+s on a new-item description issued an API request, want none (the item does not exist yet)")
	}
	if m.editorOn {
		t.Error("ctrl+s left the description editor open")
	}
	if !m.creatingItem {
		t.Error("ctrl+s dropped the review step instead of returning to it")
	}
	if m.newItemDescription != "<p>Steps to reproduce</p>" {
		t.Fatalf("newItemDescription = %q, want it saved as HTML", m.newItemDescription)
	}
	if !strings.Contains(m.viewNewItemReview(), "Steps to reproduce") {
		t.Error("the review step does not show the description that was just set")
	}
}

// TestCreateWorkItemIncludesDescription checks the review step's "enter" sends
// description_html in the POST once one has been set via "d", and omits the field entirely
// (like assignees/labels already do) when none was.
func TestCreateWorkItemIncludesDescription(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(api.WorkItem{ID: "new-1"})
	}))
	defer srv.Close()

	m := boardFixture(1, 0, 120, 40)
	m.client = api.New(srv.URL, "tok")
	m.creatingItem = true
	m.newItemName = "Card"
	m.newItemStateID = "s1"
	m.newItemPriority = "none"

	if _, cmd := m.updateNewItemReview(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		cmd()
	}
	if _, ok := gotBody["description_html"]; ok {
		t.Errorf("POST body had description_html with none set: %+v", gotBody)
	}

	m.newItemDescription = "<p>Steps to reproduce</p>"
	if _, cmd := m.updateNewItemReview(tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		cmd()
	}
	if gotBody["description_html"] != "<p>Steps to reproduce</p>" {
		t.Errorf("description_html = %v, want the text set via the review step's d editor", gotBody["description_html"])
	}
}

// TestNewItemEditorEscCreatesNothing checks esc drops the draft without calling the API and
// without touching the board's items.
func TestNewItemEditorEscCreatesNothing(t *testing.T) {
	m := boardFixture(2, 2, 120, 40)
	m.editor = newTestEditor()

	next, _ := m.openNewItemEditor()
	m = next.(Model)
	m.editor.SetValue("Abandoned title")

	next, cmd := m.updateBoard(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if cmd != nil {
		t.Error("esc should not issue a create request")
	}
	if m.editorOn || m.editorMode != "" || m.newItemStateID != "" {
		t.Errorf("esc left the editor behind (on=%v mode=%q state=%q)", m.editorOn, m.editorMode, m.newItemStateID)
	}
	if len(m.items) != 2 {
		t.Fatalf("len(items) = %d, want unchanged 2", len(m.items))
	}
}

// frameSize is the size of a rendered frame in terminal rows and columns.
func frameSize(view string) (rows, cols int) {
	lines := strings.Split(view, "\n")
	for _, line := range lines {
		if w := lipgloss.Width(line); w > cols {
			cols = w
		}
	}
	return len(lines), cols
}

// TestViewBoardFitsTerminal is the regression test for the report this task was filed for:
// "board columns did not fit on full screen ... instead on half screen one column is shown ok".
// A board with a big list of long-titled work items must render a frame that fits the terminal
// it was given — in both dimensions, at every size, full screen included.
//
// It used to overflow both ways. Vertically, every card whose title reached the truncation
// limit wrapped onto a second line (the truncation budget ignored cardStyle's own padding)
// while the row budget still counted one row per card, so a 52-row terminal got a 73-row frame.
// Horizontally, a terminal whose per-column share landed between minColWidth and
// minColWidth+4 fell through the column-width check and rendered every column at the full
// maxColWidth, so a 120-column terminal got a 252-column frame.
func TestViewBoardFitsTerminal(t *testing.T) {
	sizes := []struct{ width, height int }{
		{230, 52}, // full screen, 1080p
		{200, 50},
		{160, 48},
		{120, 40}, // used to render 252 columns wide
		{115, 52}, // "half screen"
		{100, 30},
		{80, 24}, // the classic default
		{60, 20},
		{40, 12},
		{24, 8}, // smaller than one column: still must not exceed the terminal
	}
	for _, columns := range []int{1, 3, 6, 9, 14} {
		for _, size := range sizes {
			m := boardFixture(columns, 900, size.width, size.height)
			m.focusedCol = columns / 2
			rows, cols := frameSize(m.viewBoard())
			if rows > size.height {
				t.Errorf("%d columns on a %dx%d terminal: frame is %d rows tall, want <= %d",
					columns, size.width, size.height, rows, size.height)
			}
			if cols > size.width {
				t.Errorf("%d columns on a %dx%d terminal: frame is %d columns wide, want <= %d",
					columns, size.width, size.height, cols, size.width)
			}
		}
	}
}

// TestViewBoardFitsTerminalWithUnknownSize covers the same invariant on a terminal that never
// reports its size (m.width/m.height stay 0): the board must fit the assumed fallback size
// rather than render unbounded, which is what produced a frame thousands of lines tall.
func TestViewBoardFitsTerminalWithUnknownSize(t *testing.T) {
	m := boardFixture(6, 3000, 0, 0)
	rows, cols := frameSize(m.viewBoard())
	if rows > defaultTerminalHeight {
		t.Errorf("unknown size: frame is %d rows tall, want <= %d (defaultTerminalHeight)", rows, defaultTerminalHeight)
	}
	if cols > defaultTerminalWidth {
		t.Errorf("unknown size: frame is %d columns wide, want <= %d (defaultTerminalWidth)", cols, defaultTerminalWidth)
	}
}

// TestViewBoardFitsTerminalWithOverlays checks the row budget also covers what is drawn below
// the board: an error line, a status line and an open picker all push the columns up.
func TestViewBoardFitsTerminalWithOverlays(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*Model)
	}{
		{"error", func(m *Model) { m.err = strings.Repeat("connection reset by peer; ", 8) }},
		{"status", func(m *Model) { m.status = "Refreshing..." }},
		{"state picker", func(m *Model) { m.pickerOpen = "state" }},
		{"priority picker", func(m *Model) { m.pickerOpen = "priority" }},
		{"filter picker", func(m *Model) { m.filterOpen = "label" }},
		{"new item description editor", func(m *Model) {
			m.editor = newTestEditor()
			m.creatingItem = true
			next, _ := m.openNewItemDescriptionEditor()
			*m = next.(Model)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := boardFixture(6, 900, 120, 40)
			tc.setup(&m)
			if rows, _ := frameSize(m.viewBoard()); rows > 40 {
				t.Errorf("frame is %d rows tall on a 120x40 terminal, want <= 40", rows)
			}
		})
	}
}

// TestBoardLayoutFitsWidth pins the layout arithmetic itself: however many columns a board has,
// the columns it chooses to show must actually fit side by side in the terminal, and the
// focused column must be one of them (otherwise moving with h/l scrolls into nothing).
func TestBoardLayoutFitsWidth(t *testing.T) {
	for width := 20; width <= 300; width++ {
		for _, columns := range []int{1, 2, 3, 5, 6, 8, 12, 20} {
			for _, focused := range []int{0, columns / 2, columns - 1} {
				colWidth, first, visible := boardLayout(width, columns, focused)
				if visible < 1 || visible > columns {
					t.Fatalf("width=%d columns=%d: showed %d columns", width, columns, visible)
				}
				if colWidth < minColWidth {
					t.Fatalf("width=%d columns=%d: column width %d is below minColWidth %d", width, columns, colWidth, minColWidth)
				}
				if used := visible * (colWidth + colFrame); used > width && width >= minColWidth+colFrame {
					t.Fatalf("width=%d columns=%d: %d columns of %d need %d terminal columns", width, columns, visible, colWidth, used)
				}
				if focused < first || focused >= first+visible {
					t.Fatalf("width=%d columns=%d focused=%d: window [%d,%d) leaves the focused column off screen",
						width, columns, focused, first, first+visible)
				}
			}
		}
	}
}

// TestBoardLayoutShowsEveryColumnWhenTheyFit guards the other direction: a wide terminal must
// not collapse to a window when there is room for the whole board.
func TestBoardLayoutShowsEveryColumnWhenTheyFit(t *testing.T) {
	if _, _, visible := boardLayout(230, 6, 0); visible != 6 {
		t.Errorf("6 columns on a 230-column terminal: showed %d, want all 6", visible)
	}
	if colWidth, _, _ := boardLayout(600, 3, 0); colWidth != maxColWidth {
		t.Errorf("3 columns on a 600-column terminal: column width %d, want maxColWidth %d", colWidth, maxColWidth)
	}
	if _, _, visible := boardLayout(60, 6, 0); visible >= 6 {
		t.Errorf("6 columns on a 60-column terminal: showed %d, want a smaller window", visible)
	}
}

// TestRenderColumnRowsAreExactlyThreeLinesPerCard is the invariant the row budget is built
// on: a column of maxRows cards is maxRows*cardRows+colChromeRows rows tall, no matter how
// long the titles are. A card growing past its 2 title lines + 1 meta line here is what would
// double (or worse) the height of the whole board.
func TestRenderColumnRowsAreExactlyThreeLinesPerCard(t *testing.T) {
	for _, colWidth := range []int{minColWidth, 24, 30, maxColWidth, 110} {
		for _, maxRows := range []int{3, 10, 40} {
			m := boardFixture(1, 900, 120, 40)
			got := lipgloss.Height(m.renderColumn(0, colWidth, maxRows))
			if want := maxRows*cardRows + colChromeRows; got != want {
				t.Errorf("renderColumn(colWidth=%d, maxRows=%d) is %d rows tall, want %d",
					colWidth, maxRows, got, want)
			}
		}
	}
}

// TestCardLinesFitInsideCard checks a card's text always fits the card's own content area
// (cardStyle adds cardFrame columns of padding inside the width it is rendered at), including
// for the multi-byte titles where byte length and display width disagree, and always renders
// on exactly cardRows lines.
func TestCardLinesFitInsideCard(t *testing.T) {
	m := Model{}
	for _, cardWidth := range []int{6, 10, 18, 28, 38, 100} {
		for i, name := range longNames {
			it := api.WorkItem{SequenceID: 100000 + i, Name: name}
			lines := m.cardLines(it, cardWidth, false)
			if len(lines) != cardRows {
				t.Fatalf("cardLines(%q, cardWidth=%d) returned %d lines, want %d", name, cardWidth, len(lines), cardRows)
			}
			block := strings.Join(lines, "\n")
			for _, line := range lines {
				if got, want := lipgloss.Width(line), cardWidth-cardFrame; got > want {
					t.Errorf("cardLines(%q, cardWidth=%d) line %q is %d columns wide, want <= %d", name, cardWidth, line, got, want)
				}
			}
			if rendered := cardStyle.Width(cardWidth).Render(block); lipgloss.Height(rendered) != cardRows {
				t.Errorf("cardLines(%q, cardWidth=%d) renders on %d rows, want %d", name, cardWidth, lipgloss.Height(rendered), cardRows)
			}
		}
	}
}

// TestCardLinesFlattensMultilineNames keeps a work item whose name contains a newline from
// breaking its card out of its rows and misaligning every column beside it.
func TestCardLinesFlattensMultilineNames(t *testing.T) {
	m := Model{}
	it := api.WorkItem{SequenceID: 42, Name: "first line\nsecond line"}
	for _, line := range m.cardLines(it, 40, false) {
		if strings.ContainsAny(line, "\n\r\t") {
			t.Errorf("cardLines kept a line break: %q", line)
		}
	}
}

// TestCardLinesShowsLabelsAndPriority checks the third line surfaces what the board no longer
// has a border to spare room for elsewhere: the card's priority and label names.
func TestCardLinesShowsLabelsAndPriority(t *testing.T) {
	m := Model{labels: []api.Label{{ID: "l1", Name: "bug"}, {ID: "l2", Name: "backend"}}}
	it := api.WorkItem{SequenceID: 1, Name: "x", Priority: "urgent", Labels: []string{"l1", "l2"}}
	lines := m.cardLines(it, 60, false)
	meta := lines[2]
	if !strings.Contains(meta, "urgent") {
		t.Errorf("meta line %q does not mention the priority", meta)
	}
	if !strings.Contains(meta, "bug") || !strings.Contains(meta, "backend") {
		t.Errorf("meta line %q does not mention both labels", meta)
	}
}

// TestCardLinesSelectedCardKeepsColorsWithoutBreakingHighlight guards the highlight bug:
// cardNumberStyle, priorityLabel and labelText each call lipgloss.Style.Render, which always
// ends with a reset escape sequence that is not scoped to that call's own substring — nested
// inside a selected card (wrapped in cardSelectedStyle by renderColumn), that reset used to
// silently cancel the outer background/foreground for the rest of the line, which the previous
// fix worked around by dropping the priority/label colors entirely on a selected card. Instead,
// priorityLabelOn/labelTextOn (and cardSelectedTextStyle for the plain runs between them, see
// cardMetaLine) now restate the selected background/foreground on every segment, so a selected
// card keeps its per-segment colors exactly like an unselected one, and a reset is never
// followed by a plain, unstyled run.
func TestCardLinesSelectedCardKeepsColorsWithoutBreakingHighlight(t *testing.T) {
	// go test runs with no TTY, so lipgloss's auto-detected color profile disables ANSI output
	// entirely — force one on so this test actually exercises the styling it is checking for.
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	m := Model{labels: []api.Label{{ID: "l1", Name: "bug", Color: "#f59e0b"}}}
	it := api.WorkItem{SequenceID: 123, Name: "some title", Priority: "urgent", Labels: []string{"l1"}}

	selectedLines := m.cardLines(it, 60, true)
	unselectedLines := m.cardLines(it, 60, false)

	// Both selected and unselected meta lines (line3) must still carry the priority/label text
	// — selection must not remove them.
	for i, lines := range [][]string{selectedLines, unselectedLines} {
		meta := lines[2]
		if !strings.Contains(stripANSI(meta), "urgent") || !strings.Contains(stripANSI(meta), "bug") {
			t.Errorf("case %d: meta line %q lost the priority/label text", i, meta)
		}
	}

	var sawEscape bool
	for _, line := range selectedLines {
		if strings.ContainsRune(line, '\x1b') {
			sawEscape = true
		}
		assertNoUnstyledGapAfterReset(t, line)
	}
	if !sawEscape {
		t.Error("selected cardLines lost its per-segment styling (priority/labels) entirely")
	}
}

// stripANSI removes every embedded escape sequence, leaving only the plain text.
func stripANSI(s string) string {
	return ansi.Strip(s)
}

// assertNoUnstyledGapAfterReset fails if line contains a full reset escape sequence
// ("\x1b[0m") directly followed by a plain character instead of another escape sequence or the
// end of the string — that gap is exactly the highlight hole a nested Render call's reset used
// to leave on a selected card (see cardMetaLine/cardSelectedTextStyle).
func assertNoUnstyledGapAfterReset(t *testing.T, line string) {
	t.Helper()
	const reset = "\x1b[0m"
	for i := 0; i < len(line); {
		idx := strings.Index(line[i:], reset)
		if idx < 0 {
			return
		}
		pos := i + idx + len(reset)
		if pos < len(line) && line[pos] != '\x1b' {
			t.Errorf("line %q has an unstyled gap right after a reset at byte %d", line, pos)
		}
		i = pos
	}
}

// TestTruncateMeasuresDisplayWidth covers the truncation bug visible in the report's
// screenshot: truncating by byte length sliced multi-byte runes in half (rendering a
// replacement character mid-word) and clipped Cyrillic titles to roughly half the room they
// actually had.
func TestTruncateMeasuresDisplayWidth(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("short string should be unchanged, got %q", got)
	}
	if got := truncate("hello world", 6); got != "hello…" {
		t.Errorf("truncate(%q, 6) = %q, want %q", "hello world", got, "hello…")
	}

	cyrillic := "перевірити CFS-throttling"
	for n := 1; n <= 30; n++ {
		got := truncate(cyrillic, n)
		if !utf8.ValidString(got) {
			t.Fatalf("truncate(%q, %d) = %q, which is not valid UTF-8", cyrillic, n, got)
		}
		if w := lipgloss.Width(got); w > n {
			t.Fatalf("truncate(%q, %d) is %d columns wide", cyrillic, n, w)
		}
	}
	if got, want := truncate(cyrillic, 12), 12; lipgloss.Width(got) != want {
		t.Errorf("truncate(%q, 12) = %q (%d columns), want %d — a byte-wise clip wastes half the width",
			cyrillic, got, lipgloss.Width(got), want)
	}
}

// TestPackHintsFitsWidth keeps the footer's key hints inside the terminal: the full hint line
// is 131 columns wide, so on anything narrower the terminal wrapped it and silently took a row
// the board had already given to a column.
func TestPackHintsFitsWidth(t *testing.T) {
	for _, width := range []int{20, 40, 80, 120, 200} {
		out := packHints(boardHints, width)
		for _, line := range strings.Split(out, "\n") {
			if lipgloss.Width(line) > width {
				t.Errorf("packHints(width=%d) produced a %d-column line: %q", width, lipgloss.Width(line), line)
			}
		}
		plain := ansi.Strip(out)
		for _, hint := range boardHints {
			if want := hint[0] + "  " + hint[1]; !strings.Contains(plain, want) {
				t.Errorf("packHints(width=%d) dropped hint %q", width, want)
			}
		}
	}
}

// TestPackHintsStylesKeySeparatelyFromDescription checks the key/description split the task
// asked for: a hint's key renders in helpKeyStyle, distinct from the muted helpStyle its
// description (and the surrounding whitespace) renders in — previously the whole hint line was
// wrapped in one style, so a shortcut read in exactly the same color as the text describing it.
func TestPackHintsStylesKeySeparatelyFromDescription(t *testing.T) {
	out := packHints([][2]string{{"s", "state"}}, 80)
	want := helpKeyStyle.Render("s") + helpStyle.Render("  state")
	if out != want {
		t.Errorf("packHints one-hint output = %q, want %q", out, want)
	}
}

// TestUpdateForcesClearScreenOnBoardWithUnknownSize guards against the bug behind an earlier
// "still wrong" report: on a terminal that never reports its size (m.width/m.height stay 0 —
// the same never-reported-size terminal defaultTerminalWidth/defaultTerminalHeight exist for),
// bubbletea's own renderer only erases a line's stale tail, or drops now-unused trailing lines,
// once it knows the terminal width (see standard_renderer.go's `if r.width > 0` guard around
// EraseLineRight). With that guard permanently disabled, a board frame that shrinks between
// renders leaves the previous, wider frame's leftover characters on screen. Update must route
// around this by forcing a full ClearScreen on every board update while the size is unknown.
func TestUpdateForcesClearScreenOnBoardWithUnknownSize(t *testing.T) {
	m := Model{
		screen:    screenBoard,
		project:   api.Project{Name: "Test Project"},
		states:    []api.State{{ID: "s1", Name: "Backlog"}},
		items:     []api.WorkItem{{ID: "1", State: "s1", Name: "task"}},
		colCursor: []int{0},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd == nil {
		t.Fatal("Update on board with unknown size returned a nil Cmd, want one that clears the screen")
	}
	if got, want := cmd(), tea.ClearScreen(); got != want {
		t.Fatalf("Update on board with unknown size produced %#v, want %#v (tea.ClearScreen)", got, want)
	}
}

// TestUpdateDoesNotForceClearScreenWithKnownSize checks the workaround above stays scoped to
// the degraded unknown-size case: a terminal that reports a real size must not pay for a full
// screen clear (and the flicker that comes with it) on every keystroke.
func TestUpdateDoesNotForceClearScreenWithKnownSize(t *testing.T) {
	m := Model{
		screen:    screenBoard,
		width:     120,
		height:    40,
		project:   api.Project{Name: "Test Project"},
		states:    []api.State{{ID: "s1", Name: "Backlog"}},
		items:     []api.WorkItem{{ID: "1", State: "s1", Name: "task"}},
		colCursor: []int{0},
	}

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown}); cmd != nil {
		t.Fatalf("Update on board with known size returned a non-nil Cmd (%#v), want nil", cmd())
	}
}

// TestIDPromptOpensMatchingWorkItem covers the board's "g" (open by work item id) flow end to
// end: typing a known sequence number and pressing enter opens that work item's detail screen,
// the same way pressing enter on its card does.
func TestIDPromptOpensMatchingWorkItem(t *testing.T) {
	m := boardFixture(2, 4, 120, 40) // sequence IDs 1000..1003, see boardFixture
	m.idInput = newInput("", 12)
	m.client = api.New("http://example.invalid", "token")

	next, _ := m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = next.(Model)
	if !m.idPromptOpen {
		t.Fatal("g did not open the id prompt")
	}

	for _, r := range "1002" {
		next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if m.idPromptOpen {
		t.Error("the id prompt stayed open after a match")
	}
	if m.screen != screenDetail {
		t.Fatalf("screen = %v, want screenDetail", m.screen)
	}
	if m.detailItem == nil || m.detailItem.SequenceID != 1002 {
		t.Fatalf("detailItem = %+v, want work item #1002", m.detailItem)
	}
}

// TestIDPromptReportsNotFoundAndEscCancels covers the two ways out of the prompt that do not
// open anything: a number the board has not loaded, and esc.
func TestIDPromptReportsNotFoundAndEscCancels(t *testing.T) {
	m := boardFixture(2, 4, 120, 40)
	m.idInput = newInput("", 12)
	m.idPromptOpen = true
	m.idInput.Focus()
	m.idInput.SetValue("9999")

	next, _ := m.updateBoard(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.idPromptOpen {
		t.Error("the id prompt closed on a number that is not on the board")
	}
	if m.screen == screenDetail {
		t.Error("a not-found id must not open the detail screen")
	}
	if m.err == "" {
		t.Error("a not-found id must report an error")
	}

	next, _ = m.updateBoard(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.idPromptOpen {
		t.Error("esc did not close the id prompt")
	}
	if m.screen == screenDetail {
		t.Error("esc must not open anything")
	}
}

// TestApplyBoardExtrasCachesPerProjectAndIgnoresStaleAnswers covers the assignee-picker bug:
// members (and labels) used to be wiped to nil on every board load and re-fetched from
// scratch, so a slow request, a transient error (fetchBoardExtras discards ListMembers'
// error), or simply the request still being in flight left the picker empty. applyBoardExtras
// must instead remember the last answer per project (extrasCache) and only apply an incoming
// one to the model when it is still for the project on screen.
func TestApplyBoardExtrasCachesPerProjectAndIgnoresStaleAnswers(t *testing.T) {
	m := boardFixture(1, 1, 120, 40)
	m.project.ID = "proj-a"
	members := []api.Member{{ID: "u1", DisplayName: "jane"}}

	m.applyBoardExtras(boardExtrasMsg{projectID: "proj-a", members: members})
	if len(m.members) != 1 || m.members[0].ID != "u1" {
		t.Fatalf("members = %+v, want the fetched answer applied", m.members)
	}
	if entry, ok := m.extrasCache["proj-a"]; !ok || len(entry.members) != 1 {
		t.Errorf("extrasCache[proj-a] = %+v, want the answer cached", entry)
	}

	// The user switches to another project before a second, slower answer for proj-a lands.
	m.project.ID = "proj-b"
	m.members = nil
	m.applyBoardExtras(boardExtrasMsg{projectID: "proj-a", members: []api.Member{{ID: "u2"}}})
	if m.members != nil {
		t.Errorf("a stale answer for a project the user left overwrote members on screen: %+v", m.members)
	}
	// It still updates the cache, so proj-a shows the newer copy next time it is opened.
	if entry := m.extrasCache["proj-a"]; len(entry.members) != 1 || entry.members[0].ID != "u2" {
		t.Errorf("extrasCache[proj-a] did not pick up the newer answer: %+v", entry)
	}
}

// TestExtrasStale covers the 5-minute refresh floor: missing or old enough to need a refetch,
// fresh enough not to.
func TestExtrasStale(t *testing.T) {
	m := boardFixture(1, 1, 120, 40)
	m.project.ID = "proj-a"
	if !m.extrasStale("proj-a") {
		t.Error("a project with no cache entry at all must be reported stale")
	}

	m.applyBoardExtras(boardExtrasMsg{projectID: "proj-a", members: []api.Member{{ID: "u1"}}})
	if m.extrasStale("proj-a") {
		t.Error("a just-fetched cache entry must not be reported stale")
	}

	entry := m.extrasCache["proj-a"]
	entry.fetchedAt = time.Now().Add(-extrasCacheTTL - time.Second)
	m.extrasCache["proj-a"] = entry
	if !m.extrasStale("proj-a") {
		t.Error("a cache entry older than extrasCacheTTL must be reported stale")
	}
}

// TestHandleBoardDataUsesCachedExtrasImmediately covers reopening a project already visited
// this session: the assignee/label pickers must show its last-known members/labels straight
// away rather than going empty until a fresh fetch lands.
func TestHandleBoardDataUsesCachedExtrasImmediately(t *testing.T) {
	m := boardFixture(1, 1, 120, 40)
	m.project = api.Project{ID: "proj-a"}
	m.applyBoardExtras(boardExtrasMsg{
		projectID: "proj-a",
		members:   []api.Member{{ID: "u1", DisplayName: "jane"}},
		labels:    []api.Label{{ID: "l1", Name: "bug"}},
	})
	m.members, m.labels = nil, nil // simulate the reset a prior board load left behind

	next, _ := m.handleBoardData(boardDataMsg{states: []api.State{{ID: "s1"}}})
	m = next.(Model)
	if len(m.members) != 1 || m.members[0].ID != "u1" {
		t.Errorf("members = %+v, want the cached entry applied on reopen", m.members)
	}
	if len(m.labels) != 1 || m.labels[0].ID != "l1" {
		t.Errorf("labels = %+v, want the cached entry applied on reopen", m.labels)
	}
}
