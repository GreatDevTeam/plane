package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
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

func (m Model) pickerOptionCount() int {
	if m.pickerOpen == "state" {
		return len(m.states)
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
	if m.pickerOpen == "priority" {
		title = "Change priority"
	}
	out := columnHeaderStyle.Render(title) + "\n"
	if m.pickerOpen == "state" {
		for i, st := range m.states {
			out += pickerLine(st.Name, i == m.pickerIdx)
		}
	} else {
		for i, p := range api.Priorities {
			out += pickerLine(p, i == m.pickerIdx)
		}
	}
	out += helpStyle.Render("j/k  move    enter  apply    esc  cancel")
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
	for i := range m.items {
		if m.items[i].ID == msg.item.ID {
			m.items[i] = *msg.item
			break
		}
	}
	if m.detailItem != nil && m.detailItem.ID == msg.item.ID {
		m.detailItem = msg.item
	}
	return m, nil
}
