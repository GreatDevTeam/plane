package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

// TestRenderColumnClipsBigListWithUnknownHeight guards against the board rendering
// unbounded output when it has a big list of tasks but m.height hasn't been set yet (no
// WindowSizeMsg has arrived, or the terminal never reports one). Before the
// defaultTerminalHeight fallback, an unset height skipped row-clipping entirely and
// produced a frame thousands of lines tall that no terminal could show.
func TestRenderColumnClipsBigListWithUnknownHeight(t *testing.T) {
	var items []api.WorkItem
	for i := 0; i < 3000; i++ {
		items = append(items, api.WorkItem{ID: fmt.Sprintf("id-%d", i), SequenceID: i, Name: "task", State: "s1"})
	}
	m := Model{
		states:    []api.State{{ID: "s1", Name: "Backlog"}},
		items:     items,
		colCursor: []int{0},
	}

	out := m.renderColumn(0, 40)
	lines := strings.Split(out, "\n")
	if len(lines) > defaultTerminalHeight {
		t.Fatalf("renderColumn with unknown height produced %d lines, want <= %d (defaultTerminalHeight)", len(lines), defaultTerminalHeight)
	}
}

// TestViewBoardFallsBackToNarrowWithUnknownWidth guards against the companion bug to
// TestRenderColumnClipsBigListWithUnknownHeight: on a terminal that never reports its size,
// m.width stays 0 just like m.height did. Before the defaultTerminalWidth fallback, the
// narrow-terminal check (`m.width > 0 && ...`) read width==0 as "not narrow" and rendered
// every column at full maxColWidth side by side regardless of the real (unknown) terminal
// width, which line-wrapped the whole board into an unreadable mess. With 5 states and an
// unset width, the board must fall back to the narrow single-column layout — i.e. render
// exactly one bordered column, not five joined side by side.
func TestViewBoardFallsBackToNarrowWithUnknownWidth(t *testing.T) {
	m := Model{
		project: api.Project{Name: "Test Project"},
		states: []api.State{
			{ID: "s1", Name: "Backlog"}, {ID: "s2", Name: "In Progress"},
			{ID: "s3", Name: "Review"}, {ID: "s4", Name: "Done"}, {ID: "s5", Name: "Cancelled"},
		},
		items:     []api.WorkItem{{ID: "1", State: "s1", Name: "task"}},
		colCursor: []int{0, 0, 0, 0, 0},
	}

	out := m.viewBoard()
	if got := strings.Count(out, "╭"); got != 1 {
		t.Fatalf("viewBoard with unknown width rendered %d columns, want 1 (narrow fallback)", got)
	}
}

// TestUpdateForcesClearScreenOnBoardWithUnknownSize guards against the bug behind the
// remaining "still wrong" report: on a terminal that never reports its size (m.width/m.height
// stay 0 — the same never-reported-size terminal defaultTerminalWidth/defaultTerminalHeight
// exist for), bubbletea's own renderer only erases a line's stale tail, or drops now-unused
// trailing lines, once it knows the terminal width (see standard_renderer.go's `if r.width >
// 0` guard around EraseLineRight). With that guard permanently disabled, a board frame that
// shrinks between renders leaves the previous, wider frame's leftover characters on screen —
// producing the split, overlapping card lists the report's screenshot showed. Update must
// route around this by forcing a full ClearScreen on every board update while the size is
// unknown.
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

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("short string should be unchanged, got %q", got)
	}
	if got := truncate("hello world", 6); got != "hello…" {
		t.Errorf("truncate(11 chars, 6) = %q, want %q", got, "hello…")
	}
}
