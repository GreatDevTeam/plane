package tui

import (
	"fmt"
	"strings"
	"testing"

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

	out := m.renderColumn(0, 40, false)
	lines := strings.Split(out, "\n")
	if len(lines) > defaultTerminalHeight {
		t.Fatalf("renderColumn with unknown height produced %d lines, want <= %d (defaultTerminalHeight)", len(lines), defaultTerminalHeight)
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
