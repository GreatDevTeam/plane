package tui

import (
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

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("short string should be unchanged, got %q", got)
	}
	if got := truncate("hello world", 6); got != "hello…" {
		t.Errorf("truncate(11 chars, 6) = %q, want %q", got, "hello…")
	}
}
