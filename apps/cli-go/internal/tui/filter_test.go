package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// TestOpenFilterPickerSelectsActiveStateOrPriority checks the "S"/"Y" (filter by state/filter
// by priority) pickers open with the board's current filter value highlighted — the same
// convention openFilterPicker already follows for assignee/label.
func TestOpenFilterPickerSelectsActiveStateOrPriority(t *testing.T) {
	m := boardFixture(2, 0, 120, 40)

	m.openFilterPicker("state")
	if m.filterOpen != "state" || m.filterIdx != 0 {
		t.Fatalf("openFilterPicker(state) with no active filter: filterIdx=%d, want 0 (All)", m.filterIdx)
	}

	m.filterState = m.states[1].ID
	m.openFilterPicker("state")
	if m.filterIdx != 2 {
		t.Fatalf("openFilterPicker(state) with states[1] active: filterIdx=%d, want 2", m.filterIdx)
	}

	m.filterPriority = "high"
	m.openFilterPicker("priority")
	want := 0
	for i, p := range api.Priorities {
		if p == "high" {
			want = i + 1
			break
		}
	}
	if m.filterIdx != want {
		t.Fatalf("openFilterPicker(priority) with high active: filterIdx=%d, want %d", m.filterIdx, want)
	}
}

// TestUpdateFilterPickerAppliesStateAndPriority checks enter on the state/priority filter
// pickers writes m.filterState/m.filterPriority (not m.filterAssignee/m.filterLabel), and
// that picking the leading "All" entry clears it back to "".
func TestUpdateFilterPickerAppliesStateAndPriority(t *testing.T) {
	m := boardFixture(2, 0, 120, 40)

	m.filterOpen = "state"
	m.filterIdx = 2 // states[1], +1 for "All"
	next, _ := m.updateFilterPicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.filterState != m.states[1].ID || m.filterOpen != "" {
		t.Fatalf("updateFilterPicker(state) enter: filterState=%q filterOpen=%q, want %q/\"\"", m.filterState, m.filterOpen, m.states[1].ID)
	}

	m.filterOpen = "priority"
	m.filterIdx = 1 // api.Priorities[0], +1 for "All"
	next, _ = m.updateFilterPicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.filterPriority != api.Priorities[0] {
		t.Fatalf("updateFilterPicker(priority) enter: filterPriority=%q, want %q", m.filterPriority, api.Priorities[0])
	}

	m.filterOpen = "state"
	m.filterIdx = 0 // "All"
	next, _ = m.updateFilterPicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.filterState != "" {
		t.Fatalf("updateFilterPicker(state) picking All: filterState=%q, want \"\"", m.filterState)
	}
}
