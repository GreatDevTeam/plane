package tui

import tea "github.com/charmbracelet/bubbletea"

// openFilterPicker opens a list of every assignee (or label) on the project, plus a
// leading "All" entry, so the current filter's value stays selected.
func (m *Model) openFilterPicker(kind string) {
	m.filterOpen = kind
	m.filterIdx = 0
	active := m.filterAssignee
	if kind == "label" {
		active = m.filterLabel
	}
	if active == "" {
		return
	}
	for i, id := range m.filterOptionIDs() {
		if id == active {
			m.filterIdx = i + 1 // +1 for the leading "All" entry
			break
		}
	}
}

// filterOptionIDs returns the selectable IDs (members or labels) for the open filter kind,
// in the same order as filterOptionLabels.
func (m Model) filterOptionIDs() []string {
	if m.filterOpen == "assignee" {
		ids := make([]string, len(m.members))
		for i, mem := range m.members {
			ids[i] = mem.ID
		}
		return ids
	}
	ids := make([]string, len(m.labels))
	for i, l := range m.labels {
		ids[i] = l.ID
	}
	return ids
}

func (m Model) filterOptionLabels() []string {
	if m.filterOpen == "assignee" {
		out := make([]string, len(m.members))
		for i, mem := range m.members {
			out[i] = mem.Name()
		}
		return out
	}
	out := make([]string, len(m.labels))
	for i, l := range m.labels {
		out[i] = l.Name
	}
	return out
}

func (m Model) updateFilterPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	count := len(m.filterOptionIDs()) + 1 // +1 for "All"
	switch key.String() {
	case "esc", "q":
		m.filterOpen = ""
	case "up", "k":
		if m.filterIdx > 0 {
			m.filterIdx--
		}
	case "down", "j":
		if m.filterIdx < count-1 {
			m.filterIdx++
		}
	case "enter":
		ids := m.filterOptionIDs()
		var value string
		if m.filterIdx > 0 && m.filterIdx-1 < len(ids) {
			value = ids[m.filterIdx-1]
		}
		if m.filterOpen == "assignee" {
			m.filterAssignee = value
		} else {
			m.filterLabel = value
		}
		m.filterOpen = ""
	}
	return m, nil
}

func (m Model) viewFilterPicker() string {
	title := "Filter by assignee"
	if m.filterOpen == "label" {
		title = "Filter by label"
	}
	out := columnHeaderStyle.Render(title) + "\n"
	out += pickerLine("All", m.filterIdx == 0)
	for i, label := range m.filterOptionLabels() {
		out += pickerLine(label, m.filterIdx == i+1)
	}
	out += helpStyle.Render("j/k  move    enter  apply    esc  cancel")
	return focusedInputStyle.Render(out)
}

// activeFilterSummary renders a short "filtered by ..." note for the board header, or ""
// when no filter is active.
func (m Model) activeFilterSummary() string {
	var parts []string
	if m.filterAssignee != "" {
		for _, mem := range m.members {
			if mem.ID == m.filterAssignee {
				parts = append(parts, "assignee: "+mem.Name())
				break
			}
		}
	}
	if m.filterLabel != "" {
		for _, l := range m.labels {
			if l.ID == m.filterLabel {
				parts = append(parts, "label: "+l.Name)
				break
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	out := "filtered by "
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
