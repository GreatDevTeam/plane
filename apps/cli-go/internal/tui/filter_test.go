package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
	"github.com/makeplane/plane/apps/cli-go/internal/config"
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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
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

// TestBoardFiltersPersistAcrossReload covers "filters on board must persist until changed":
// applying a filter used to only live in memory, so reloading the board — e.g. switching away
// to another project and back, or restarting — silently dropped it. Applying a filter must now
// save it (see config.BoardFilter), and loading the board must restore it instead of resetting
// every filter to "All".
func TestBoardFiltersPersistAcrossReload(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := boardFixture(2, 0, 120, 40)
	m.project = api.Project{ID: "proj-1"}

	m.filterOpen = "state"
	m.filterIdx = 2 // states[1], +1 for "All"
	next, _ := m.updateFilterPicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	wantState := m.states[1].ID

	saved, err := config.Load()
	if err != nil {
		t.Fatalf("loading the saved config: %v", err)
	}
	if saved.BoardFilterFor("proj-1").State != wantState {
		t.Fatalf("persisted state filter = %q, want %q", saved.BoardFilterFor("proj-1").State, wantState)
	}

	// Simulate the board reloading (switching away and back, or a restart): a fresh Model
	// loads the same saved config and must come up with the filter already applied.
	m2 := boardFixture(2, 0, 120, 40)
	m2.project = api.Project{ID: "proj-1"}
	m2.cfg = saved
	next, _ = m2.handleBoardData(boardDataMsg{states: m.states})
	m2 = next.(Model)
	if m2.filterState != wantState {
		t.Errorf("reloaded board filterState = %q, want %q (filter did not persist)", m2.filterState, wantState)
	}
}
