package tui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// TestOpenAssigneePickerSelectsCurrentAssignee checks the "A" (change assignee) picker opens
// with the active item's current assignee highlighted — "Unassigned" when it has none — the
// same convention openStatePicker/openPriorityPicker already follow for state/priority. It
// covers both screens the picker is reachable from: the board (via the selected card) and the
// detail screen (via the open item), since both funnel through activeItem().
func TestOpenAssigneePickerSelectsCurrentAssignee(t *testing.T) {
	members := []api.Member{{ID: "u1", DisplayName: "Alice"}, {ID: "u2", DisplayName: "Bob"}}

	t.Run("board, unassigned", func(t *testing.T) {
		m := boardFixture(1, 1, 120, 40)
		m.members = members
		m.openAssigneePicker()
		if m.pickerOpen != "assignee" || m.pickerIdx != 0 {
			t.Fatalf("openAssigneePicker: pickerOpen=%q pickerIdx=%d, want assignee/0 (Unassigned)", m.pickerOpen, m.pickerIdx)
		}
	})

	t.Run("board, currently assigned", func(t *testing.T) {
		m := boardFixture(1, 1, 120, 40)
		m.members = members
		m.items[0].Assignees = []string{"u2"}
		m.openAssigneePicker()
		if m.pickerOpen != "assignee" || m.pickerIdx != 2 {
			t.Fatalf("openAssigneePicker: pickerIdx=%d, want 2 (Bob, +1 for the leading Unassigned entry)", m.pickerIdx)
		}
	})

	t.Run("detail screen", func(t *testing.T) {
		m := boardFixture(1, 1, 120, 40)
		m.members = members
		m.screen = screenDetail
		item := m.items[0]
		item.Assignees = []string{"u1"}
		m.detailItem = &item
		m.openAssigneePicker()
		if m.pickerOpen != "assignee" || m.pickerIdx != 1 {
			t.Fatalf("openAssigneePicker on the detail screen: pickerIdx=%d, want 1 (Alice)", m.pickerIdx)
		}
	})
}

// TestUpdatePickerAssigneeBuildsSingleAssigneePatch checks enter on the assignee picker PATCHes
// {"assignees": [...]} from the selection (a single choice replaces the whole list, the same
// as new-item-assignee sets one), rather than the {"state": ...}/{"priority": ...} shape the
// picker also drives — and that picking the leading "Unassigned" entry clears it.
func TestUpdatePickerAssigneeBuildsSingleAssigneePatch(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(api.WorkItem{ID: "id-0"})
	}))
	defer srv.Close()

	m := boardFixture(1, 1, 120, 40)
	m.client = api.New(srv.URL, "tok")
	m.members = []api.Member{{ID: "u1", DisplayName: "Alice"}, {ID: "u2", DisplayName: "Bob"}}
	m.pickerOpen = "assignee"
	m.pickerIdx = 2 // Bob (+1 for the leading Unassigned entry)

	next, cmd := m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("enter on the assignee picker returned no command")
	}
	if msg, ok := cmd().(workItemUpdatedMsg); !ok || msg.err != nil {
		t.Fatalf("updateWorkItem command = %+v", msg)
	}
	if ids, ok := gotBody["assignees"].([]any); !ok || len(ids) != 1 || ids[0] != "u2" {
		t.Fatalf("PATCH body assignees = %v, want [\"u2\"]", gotBody["assignees"])
	}
	// The status names who it's assigning to (rather than a generic "Updating...") so this is
	// distinguishable, in the moment, from the board's own "a" filter key — see the comment on
	// this case in applyPickerEnter.
	if m.status != "Assigning to Bob..." {
		t.Errorf("status = %q, want it to name the assignee being set", m.status)
	}

	// Picking "Unassigned" (index 0) clears the list rather than leaving the previous
	// assignee untouched.
	gotBody = nil
	m.pickerIdx = 0
	next, cmd = m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if msg, ok := cmd().(workItemUpdatedMsg); !ok || msg.err != nil {
		t.Fatalf("updateWorkItem command = %+v", msg)
	}
	if ids, ok := gotBody["assignees"].([]any); !ok || len(ids) != 0 {
		t.Fatalf("PATCH body assignees = %v, want []", gotBody["assignees"])
	}
}

// TestAssigneePickerIsSearchable checks the assignee picker is a type-to-filter search box like
// the state/labels pickers (updateSearchablePicker): typing narrows m.assigneePickerRows() to
// matching members (plus "Unassigned" while it still matches the query), and enter applies
// whatever row ends up under the cursor after the filter, not the original unfiltered index.
func TestAssigneePickerIsSearchable(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(api.WorkItem{ID: "id-0"})
	}))
	defer srv.Close()

	m := boardFixture(1, 1, 120, 40)
	m.client = api.New(srv.URL, "tok")
	m.members = []api.Member{{ID: "u1", DisplayName: "Alice"}, {ID: "u2", DisplayName: "Bob"}}
	m.openAssigneePicker()

	// Typing "bo" narrows the list to just Bob (Unassigned no longer matches either).
	next, _ := m.updatePicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = next.(Model)
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = next.(Model)
	if rows := m.assigneePickerRows(); len(rows) != 1 || rows[0] != "u2" {
		t.Fatalf("assigneePickerRows after typing \"bo\" = %v, want just u2", rows)
	}
	if m.pickerIdx != 0 {
		t.Fatalf("pickerIdx = %d after the list narrowed to 1 entry, want it clamped to 0", m.pickerIdx)
	}

	_, cmd := m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on the filtered assignee picker returned no command")
	}
	if msg, ok := cmd().(workItemUpdatedMsg); !ok || msg.err != nil {
		t.Fatalf("updateWorkItem command = %+v", msg)
	}
	if ids, ok := gotBody["assignees"].([]any); !ok || len(ids) != 1 || ids[0] != "u2" {
		t.Fatalf("PATCH body assignees = %v, want [\"u2\"]", gotBody["assignees"])
	}
}

// TestOpenLabelPickerSeedsSelectionFromItem checks the "T" (change labels) picker opens with
// the active item's current labels pre-toggled, the multi-select equivalent of
// TestOpenAssigneePickerSelectsCurrentAssignee.
func TestOpenLabelPickerSeedsSelectionFromItem(t *testing.T) {
	m := boardFixture(1, 1, 120, 40)
	m.labels = []api.Label{{ID: "l1", Name: "bug"}, {ID: "l2", Name: "urgent"}}
	m.items[0].Labels = []string{"l2"}

	m.openLabelPicker()
	if m.pickerOpen != "labels" {
		t.Fatalf("openLabelPicker: pickerOpen=%q, want labels", m.pickerOpen)
	}
	if m.labelPickerSelected["l1"] || !m.labelPickerSelected["l2"] {
		t.Fatalf("openLabelPicker: selected=%v, want only l2 toggled on", m.labelPickerSelected)
	}
}

// TestUpdatePickerLabelsTogglesAndBuildsMultiLabelPatch checks space toggles entries in the
// working set (without PATCHing anything) and enter PATCHes {"labels": [...]} from whatever
// ended up toggled on, including an empty selection marshaling to [] rather than null (the
// same convention TestUpdatePickerAssigneeBuildsSingleAssigneePatch checks for assignees).
func TestUpdatePickerLabelsTogglesAndBuildsMultiLabelPatch(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(api.WorkItem{ID: "id-0"})
	}))
	defer srv.Close()

	m := boardFixture(1, 1, 120, 40)
	m.client = api.New(srv.URL, "tok")
	m.labels = []api.Label{{ID: "l1", Name: "bug"}, {ID: "l2", Name: "urgent"}, {ID: "l3", Name: "docs"}}
	m.openLabelPicker() // nothing selected: item has no labels

	// Toggle l1 on, move to l3 and toggle it on too.
	next, _ := m.updatePicker(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(Model)
	m.pickerIdx = 2
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeySpace})
	m = next.(Model)
	if !m.labelPickerSelected["l1"] || m.labelPickerSelected["l2"] || !m.labelPickerSelected["l3"] {
		t.Fatalf("after toggling l1 and l3: selected=%v", m.labelPickerSelected)
	}

	_, cmd := m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on the labels picker returned no command")
	}
	if msg, ok := cmd().(workItemUpdatedMsg); !ok || msg.err != nil {
		t.Fatalf("updateWorkItem command = %+v", msg)
	}
	ids, ok := gotBody["labels"].([]any)
	if !ok || len(ids) != 2 || ids[0] != "l1" || ids[1] != "l3" {
		t.Fatalf("PATCH body labels = %v, want [\"l1\",\"l3\"]", gotBody["labels"])
	}

	// Toggling nothing on at all PATCHes an empty list, not null.
	gotBody = nil
	m.openLabelPicker()
	_, cmd = m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	if msg, ok := cmd().(workItemUpdatedMsg); !ok || msg.err != nil {
		t.Fatalf("updateWorkItem command = %+v", msg)
	}
	if ids, ok := gotBody["labels"].([]any); !ok || len(ids) != 0 {
		t.Fatalf("PATCH body labels = %v, want []", gotBody["labels"])
	}
}

// TestStatePickerSearchFiltersAndApplies checks typing into the state picker narrows the list
// (case-insensitively) and enter still applies whatever the cursor lands on within that
// filtered list, not the unfiltered m.states — the bug a naive filtered-view-only-at-render-time
// implementation would have.
func TestStatePickerSearchFiltersAndApplies(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(api.WorkItem{ID: "id-0"})
	}))
	defer srv.Close()

	m := boardFixture(1, 1, 120, 40)
	m.client = api.New(srv.URL, "tok")
	m.states = []api.State{{ID: "s1", Name: "Todo"}, {ID: "s2", Name: "In Progress"}, {ID: "s3", Name: "Done"}}
	m.items[0].State = "s1" // keep the fixture's one card in the (now renamed) focused column
	m.openStatePicker()

	for _, r := range "prog" {
		next, _ := m.updatePicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	opts := m.filteredStates()
	if len(opts) != 1 || opts[0].ID != "s2" {
		t.Fatalf("filteredStates() after typing %q = %v, want only s2 (In Progress)", m.pickerSearch.Value(), opts)
	}
	if m.pickerOptionCount() != 1 {
		t.Fatalf("pickerOptionCount() = %d, want 1 while filtered", m.pickerOptionCount())
	}

	next, cmd := m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("enter on the state picker returned no command")
	}
	if msg, ok := cmd().(workItemUpdatedMsg); !ok || msg.err != nil {
		t.Fatalf("updateWorkItem command = %+v", msg)
	}
	if gotBody["state"] != "s2" {
		t.Fatalf("PATCH body state = %v, want s2", gotBody["state"])
	}
}

// TestPriorityPickerSearchFiltersAndApplies is TestStatePickerSearchFiltersAndApplies for the
// priority picker: typing narrows api.Priorities to matching entries and enter PATCHes whatever
// ends up under the cursor after the filter.
func TestPriorityPickerSearchFiltersAndApplies(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(api.WorkItem{ID: "id-0"})
	}))
	defer srv.Close()

	m := boardFixture(1, 1, 120, 40)
	m.client = api.New(srv.URL, "tok")
	m.openPriorityPicker()

	for _, r := range "urg" {
		next, _ := m.updatePicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	opts := m.filteredPriorities()
	if len(opts) != 1 || opts[0] != "urgent" {
		t.Fatalf("filteredPriorities() after typing %q = %v, want only urgent", m.pickerSearch.Value(), opts)
	}
	if m.pickerOptionCount() != 1 {
		t.Fatalf("pickerOptionCount() = %d, want 1 while filtered", m.pickerOptionCount())
	}

	next, cmd := m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("enter on the priority picker returned no command")
	}
	if msg, ok := cmd().(workItemUpdatedMsg); !ok || msg.err != nil {
		t.Fatalf("updateWorkItem command = %+v", msg)
	}
	if gotBody["priority"] != "urgent" {
		t.Fatalf("PATCH body priority = %v, want urgent", gotBody["priority"])
	}
}

// TestNewItemPriorityPickerSearchSetsNewItemPriority checks the same search box works for the
// board's new-work-item review step, where a choice lands in m.newItemPriority instead of
// PATCHing anything (there is nothing to PATCH yet).
func TestNewItemPriorityPickerSearchSetsNewItemPriority(t *testing.T) {
	m := boardFixture(1, 1, 120, 40)
	m.creatingItem = true
	m.openNewItemPriorityPicker()

	for _, r := range "high" {
		next, _ := m.updatePicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	opts := m.filteredPriorities()
	if len(opts) != 1 || opts[0] != "high" {
		t.Fatalf("filteredPriorities() after typing %q = %v, want only high", m.pickerSearch.Value(), opts)
	}

	next, _ := m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.newItemPriority != "high" {
		t.Fatalf("newItemPriority = %q, want high", m.newItemPriority)
	}
	if m.pickerOpen != "" {
		t.Error("picker stayed open after enter")
	}
}

// TestLabelsPickerSearchNarrowsListAndClampsCursor checks the labels picker's search box
// narrows the list the same way, and that the cursor is clamped back onto the (now shorter)
// filtered list rather than left pointing past its end.
func TestLabelsPickerSearchNarrowsListAndClampsCursor(t *testing.T) {
	m := boardFixture(1, 1, 120, 40)
	m.labels = []api.Label{{ID: "l1", Name: "bug"}, {ID: "l2", Name: "urgent"}, {ID: "l3", Name: "docs"}}
	m.openLabelPicker()
	m.pickerIdx = 2 // "docs", the last entry

	for _, r := range "bu" {
		next, _ := m.updatePicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	opts := m.filteredLabels()
	if len(opts) != 1 || opts[0].ID != "l1" {
		t.Fatalf("filteredLabels() after typing %q = %v, want only l1 (bug)", m.pickerSearch.Value(), opts)
	}
	if m.pickerIdx != 0 {
		t.Errorf("pickerIdx = %d after the list narrowed to 1 entry, want it clamped to 0", m.pickerIdx)
	}
}

// TestColorTargetPickerOpensColorPromptWithCurrentColor checks picking a state or a label from
// the "C" color-settings picker opens the hex-entry prompt pre-filled with its current
// effective color (the state's own API color when no override is set yet).
func TestColorTargetPickerOpensColorPromptWithCurrentColor(t *testing.T) {
	m := boardFixture(1, 1, 120, 40)
	m.project = api.Project{ID: "proj-1"}
	m.states = []api.State{{ID: "s1", Name: "Todo", Color: "#16a34a"}}
	m.labels = []api.Label{{ID: "l1", Name: "bug", Color: "#f59e0b"}}
	m.openColorTargetPicker()
	if m.pickerOpen != "color-target" {
		t.Fatalf("openColorTargetPicker: pickerOpen=%q, want color-target", m.pickerOpen)
	}

	// pickerIdx 0 is the state.
	next, _ := m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.colorPromptOpen || m.colorTargetKind != "state" || m.colorTargetID != "s1" {
		t.Fatalf("state row: colorPromptOpen=%v kind=%q id=%q, want open/state/s1", m.colorPromptOpen, m.colorTargetKind, m.colorTargetID)
	}
	if got := m.colorInput.Value(); got != "16a34a" {
		t.Errorf("colorInput = %q, want the state's own color 16a34a", got)
	}
	m = m.closeColorPrompt()

	// pickerIdx 1 is the label (index len(states)+0).
	m.openColorTargetPicker()
	m.pickerIdx = 1
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.colorPromptOpen || m.colorTargetKind != "label" || m.colorTargetID != "l1" {
		t.Fatalf("label row: colorPromptOpen=%v kind=%q id=%q, want open/label/l1", m.colorPromptOpen, m.colorTargetKind, m.colorTargetID)
	}
	if got := m.colorInput.Value(); got != "f59e0b" {
		t.Errorf("colorInput = %q, want the label's own color f59e0b", got)
	}
	m = m.closeColorPrompt()

	// pickerIdx 2 is the first priority row (index len(states)+len(labels)+0), "urgent".
	m.openColorTargetPicker()
	m.pickerIdx = 2
	next, _ = m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.colorPromptOpen || m.colorTargetKind != "priority" || m.colorTargetID != "urgent" {
		t.Fatalf("priority row: colorPromptOpen=%v kind=%q id=%q, want open/priority/urgent", m.colorPromptOpen, m.colorTargetKind, m.colorTargetID)
	}
}

// TestUpdateColorPromptSavesOverrideAndAppliesToEffectiveColor covers the whole round trip:
// enter on a valid hex persists it via config.SetStateColor, and effectiveStateColor picks it
// up over the state's own API color on the very next call.
func TestUpdateColorPromptSavesOverrideAndAppliesToEffectiveColor(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := boardFixture(1, 1, 120, 40)
	m.project = api.Project{ID: "proj-1"}
	st := api.State{ID: "s1", Name: "Todo", Color: "#16a34a"}
	m.states = []api.State{st}
	m.openColorPrompt("state", "s1", "#16a34a")
	m.colorInput.SetValue("ff8800")

	next, _ := m.updateColorPrompt(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.colorPromptOpen {
		t.Fatal("the color prompt stayed open after a valid hex")
	}
	if got := m.effectiveStateColor(st); got != "#ff8800" {
		t.Fatalf("effectiveStateColor after saving an override = %q, want #ff8800", got)
	}

	// An invalid hex is rejected and the prompt stays open.
	m.openColorPrompt("state", "s1", "#ff8800")
	m.colorInput.SetValue("not-a-color")
	next, _ = m.updateColorPrompt(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.colorPromptOpen {
		t.Error("the color prompt closed after an invalid hex")
	}
	if m.err == "" {
		t.Error("an invalid hex must report an error")
	}
}

// TestUpdateColorPromptSavesPriorityOverride is
// TestUpdateColorPromptSavesOverrideAndAppliesToEffectiveColor for a priority: enter on a valid
// hex persists it via config.SetPriorityColor, global rather than keyed by project like the
// state/label overrides, and effectivePriorityColor picks it up on the very next call.
func TestUpdateColorPromptSavesPriorityOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := boardFixture(1, 1, 120, 40)
	m.project = api.Project{ID: "proj-1"}
	m.openColorPrompt("priority", "urgent", "#ff0000")
	m.colorInput.SetValue("00ff00")

	next, _ := m.updateColorPrompt(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.colorPromptOpen {
		t.Fatal("the color prompt stayed open after a valid hex")
	}
	if got := m.effectivePriorityColor("urgent"); got != "#00ff00" {
		t.Fatalf("effectivePriorityColor after saving an override = %q, want #00ff00", got)
	}
	if got := m.effectivePriorityColor("high"); got == "#00ff00" {
		t.Error("a different priority must not see urgent's override")
	}
}
