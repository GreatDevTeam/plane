package tui

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

func TestColumnItemsFiltersByAssigneeAndLabel(t *testing.T) {
	m := Model{
		states: []api.State{{ID: "s1"}},
		items: []api.WorkItem{
			{ID: "1", State: "s1", Assignees: []string{"u1"}, Labels: []string{"l1"}},
			{ID: "2", State: "s1", Assignees: []string{"u2"}, Labels: []string{"l1"}},
			{ID: "3", State: "s1", Assignees: []string{"u1"}, Labels: []string{"l2"}},
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
		project:   api.Project{Name: "A Project With A Fairly Long Name"},
		states:    states,
		items:     work,
		colCursor: make([]int, columns),
		width:     width,
		height:    height,
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
			lines := m.cardLines(it, cardWidth)
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
	for _, line := range m.cardLines(it, 40) {
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
	lines := m.cardLines(it, 60)
	meta := lines[2]
	if !strings.Contains(meta, "urgent") {
		t.Errorf("meta line %q does not mention the priority", meta)
	}
	if !strings.Contains(meta, "bug") || !strings.Contains(meta, "backend") {
		t.Errorf("meta line %q does not mention both labels", meta)
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
		for _, hint := range boardHints {
			if !strings.Contains(out, hint) {
				t.Errorf("packHints(width=%d) dropped hint %q", width, hint)
			}
		}
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
