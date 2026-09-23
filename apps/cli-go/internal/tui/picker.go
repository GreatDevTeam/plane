package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
	"github.com/makeplane/plane/apps/cli-go/internal/config"
)

// activeItem returns whichever work item the currently open picker applies to: the one
// selected on the board, or the one shown on the detail screen.
func (m Model) activeItem() *api.WorkItem {
	if m.screen == screenDetail {
		return m.detailItem
	}
	return m.selectedItem()
}

func (m *Model) openStatePicker() {
	item := m.activeItem()
	if item == nil {
		return
	}
	m.pickerOpen = "state"
	m.pickerIdx = 0
	for i, st := range m.states {
		if st.ID == item.State {
			m.pickerIdx = i
			break
		}
	}
}

func (m *Model) openPriorityPicker() {
	item := m.activeItem()
	if item == nil {
		return
	}
	m.pickerOpen = "priority"
	m.pickerIdx = 0
	for i, p := range api.Priorities {
		if p == item.Priority {
			m.pickerIdx = i
			break
		}
	}
}

// openSortPicker opens the list of card orderings, with the board's current one selected.
func (m *Model) openSortPicker() {
	m.pickerOpen = "sort"
	m.pickerIdx = 0
	current := normalizeSortMode(m.sortMode)
	for i, sm := range sortModes {
		if sm.key == current {
			m.pickerIdx = i
			break
		}
	}
}

func (m Model) pickerOptionCount() int {
	switch m.pickerOpen {
	case "state":
		return len(m.states)
	case "sort":
		return len(sortModes)
	case "subissue":
		return len(m.subIssueOptions())
	}
	return len(api.Priorities)
}

func (m Model) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc", "q":
		m.pickerOpen = ""
	case "up", "k":
		if m.pickerIdx > 0 {
			m.pickerIdx--
		}
	case "down", "j":
		if m.pickerIdx < m.pickerOptionCount()-1 {
			m.pickerIdx++
		}
	case "enter":
		if m.pickerOpen == "sort" {
			// Ordering is a view setting, not a change to any work item: apply it locally
			// (and remember it for the next run) instead of PATCHing anything.
			if m.pickerIdx >= 0 && m.pickerIdx < len(sortModes) {
				m.sortMode = sortModes[m.pickerIdx].key
				m.cfg.SortMode = m.sortMode
				if err := config.Save(m.cfg); err != nil {
					m.setError(err)
				}
				m.clampBoardCursors()
			}
			m.pickerOpen = ""
			return m, nil
		}
		if m.pickerOpen == "subissue" {
			// Not a change to any work item either: this picker navigates, so enter opens
			// the chosen sub-task the same way the board's enter opens a card.
			opts := m.subIssueOptions()
			if m.pickerIdx < 0 || m.pickerIdx >= len(opts) {
				m.pickerOpen = ""
				return m, nil
			}
			return m.openWorkItem(opts[m.pickerIdx])
		}
		item := m.activeItem()
		if item == nil {
			m.pickerOpen = ""
			return m, nil
		}
		var patch map[string]any
		if m.pickerOpen == "state" {
			patch = map[string]any{"state": m.states[m.pickerIdx].ID}
		} else {
			patch = map[string]any{"priority": api.Priorities[m.pickerIdx]}
		}
		m.status = "Updating..."
		return m, updateWorkItem(m.client, m.workspaceSlug, m.project.ID, item.ID, patch)
	}
	return m, nil
}

func (m Model) viewPicker() string {
	title := "Change state"
	switch m.pickerOpen {
	case "priority":
		title = "Change priority"
	case "sort":
		title = "Order cards by"
	case "subissue":
		title = "Jump to sub-task"
	}
	out := columnHeaderStyle.Render(title) + "\n"
	hint := "j/k  move    enter  apply    esc  cancel"
	switch {
	case m.pickerOpen == "state":
		for i, st := range m.states {
			out += pickerLine(st.Name, i == m.pickerIdx)
		}
	case m.pickerOpen == "sort":
		for i, sm := range sortModes {
			out += pickerLine(sm.label, i == m.pickerIdx)
		}
	case m.pickerOpen == "subissue":
		opts := m.subIssueOptions()
		// Windowed, unlike the lists above: the number of sub-tasks is project data with no
		// upper bound, and this picker is drawn below the detail screen's two panes, so an
		// unwindowed list would push its own bottom off the terminal (see pickerWindow).
		start, count, scrolled := pickerWindow(len(opts), m.pickerIdx, m.subIssuePickerRows())
		if scrolled {
			out = columnHeaderStyle.Render(fmt.Sprintf("%s (%d-%d of %d)", title, start+1, start+count, len(opts))) + "\n"
		}
		for i := start; i < start+count; i++ {
			out += pickerLine(m.relationSummary(opts[i]), i == m.pickerIdx)
		}
		hint = "j/k  move    enter  open    esc  cancel"
	default:
		for i, p := range api.Priorities {
			out += pickerLine(p, i == m.pickerIdx)
		}
	}
	out += helpStyle.Render(hint)
	return focusedInputStyle.Render(out)
}

func pickerLine(label string, selected bool) string {
	if selected {
		return cardSelectedStyle.Render("> "+label) + "\n"
	}
	return cardStyle.Render("  "+label) + "\n"
}

func (m Model) handleWorkItemUpdated(msg workItemUpdatedMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	m.pickerOpen = ""
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	// A description edit lands here too (it is a PATCH like any other field change), so
	// close the editor once the save comes back.
	if m.editorOn && m.editorMode == "description" {
		m.closeEditor()
	}
	for i := range m.items {
		if m.items[i].ID == msg.item.ID {
			m.items[i] = *msg.item
			break
		}
	}
	if m.detailItem != nil && m.detailItem.ID == msg.item.ID {
		m.detailItem = msg.item
	}
	m.clampBoardCursors()
	return m, nil
}
