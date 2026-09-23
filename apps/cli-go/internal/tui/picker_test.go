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

	_, cmd := m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on the assignee picker returned no command")
	}
	if msg, ok := cmd().(workItemUpdatedMsg); !ok || msg.err != nil {
		t.Fatalf("updateWorkItem command = %+v", msg)
	}
	if ids, ok := gotBody["assignees"].([]any); !ok || len(ids) != 1 || ids[0] != "u2" {
		t.Fatalf("PATCH body assignees = %v, want [\"u2\"]", gotBody["assignees"])
	}

	// Picking "Unassigned" (index 0) clears the list rather than leaving the previous
	// assignee untouched.
	gotBody = nil
	m.pickerIdx = 0
	_, cmd = m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	if msg, ok := cmd().(workItemUpdatedMsg); !ok || msg.err != nil {
		t.Fatalf("updateWorkItem command = %+v", msg)
	}
	if ids, ok := gotBody["assignees"].([]any); !ok || len(ids) != 0 {
		t.Fatalf("PATCH body assignees = %v, want []", gotBody["assignees"])
	}
}
