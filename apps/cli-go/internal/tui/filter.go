package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// openFilterPicker opens a list of every assignee (or label/state/priority) on the project,
// plus a leading "All" entry, so the current filter's value stays selected. The label filter
// also resets/focuses its type-to-filter query box (see filteredFilterLabels), the project's
// label list being the one long enough to be worth searching on the board.
func (m *Model) openFilterPicker(kind string) {
	m.filterOpen = kind
	m.filterIdx = 0
	if kind == "label" {
		m.filterSearch.Reset()
		m.filterSearch.Focus()
	}
	active := m.filterActiveValue(kind)
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

// filterActiveValue returns the currently applied filter value for the given kind.
func (m Model) filterActiveValue(kind string) string {
	switch kind {
	case "assignee":
		return m.filterAssignee
	case "state":
		return m.filterState
	case "priority":
		return m.filterPriority
	default:
		return m.filterLabel
	}
}

// filterOptionIDs returns the selectable IDs for the open filter kind, in the same order as
// filterOptionLabels.
func (m Model) filterOptionIDs() []string {
	switch m.filterOpen {
	case "assignee":
		ids := make([]string, len(m.members))
		for i, mem := range m.members {
			ids[i] = mem.ID
		}
		return ids
	case "state":
		ids := make([]string, len(m.states))
		for i, st := range m.states {
			ids[i] = st.ID
		}
		return ids
	case "priority":
		return append([]string{}, api.Priorities...)
	default:
		labels := m.filteredFilterLabels()
		ids := make([]string, len(labels))
		for i, l := range labels {
			ids[i] = l.ID
		}
		return ids
	}
}

// filteredFilterLabels is m.labels narrowed by the label filter's search query (m.filterSearch),
// the same type-to-filter behavior filteredLabels (picker.go) gives the labels-editing picker.
// An empty query matches everything, so the filter picker looks and behaves exactly as it did
// before it could search.
func (m Model) filteredFilterLabels() []api.Label {
	q := strings.ToLower(strings.TrimSpace(m.filterSearch.Value()))
	if q == "" {
		return m.labels
	}
	out := make([]api.Label, 0, len(m.labels))
	for _, l := range m.labels {
		if strings.Contains(strings.ToLower(l.Name), q) {
			out = append(out, l)
		}
	}
	return out
}

func (m Model) filterOptionLabels() []string {
	switch m.filterOpen {
	case "assignee":
		out := make([]string, len(m.members))
		for i, mem := range m.members {
			out[i] = mem.Name()
		}
		return out
	case "state":
		out := make([]string, len(m.states))
		for i, st := range m.states {
			out[i] = st.Name
		}
		return out
	case "priority":
		return append([]string{}, api.Priorities...)
	default:
		labels := m.filteredFilterLabels()
		out := make([]string, len(labels))
		for i, l := range labels {
			out[i] = l.Name
		}
		return out
	}
}

func (m Model) updateFilterPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	// The label filter is a type-to-filter search box (see updateLabelFilterSearch): j/k/q are
	// ordinary characters to type there, not shortcuts, so it needs its own key handling
	// entirely separate from the plain-list assignee/state/priority filters below.
	if m.filterOpen == "label" {
		return m.updateLabelFilterSearch(key)
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
		switch m.filterOpen {
		case "assignee":
			m.filterAssignee = value
		case "state":
			m.filterState = value
		case "priority":
			m.filterPriority = value
		default:
			m.filterLabel = value
		}
		m.filterOpen = ""
	}
	return m, nil
}

// updateLabelFilterSearch drives the label filter's type-to-filter query box, the same pattern
// updateSearchablePicker (picker.go) uses for the labels-editing picker: up/down move, enter
// applies, esc cancels, everything else edits the query.
func (m Model) updateLabelFilterSearch(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.filterOpen = ""
		m.filterSearch.Blur()
		return m, nil
	case "up":
		if m.filterIdx > 0 {
			m.filterIdx--
		}
		return m, nil
	case "down":
		if m.filterIdx < len(m.filterOptionIDs()) {
			m.filterIdx++
		}
		return m, nil
	case "enter":
		ids := m.filterOptionIDs()
		var value string
		if m.filterIdx > 0 && m.filterIdx-1 < len(ids) {
			value = ids[m.filterIdx-1]
		}
		m.filterLabel = value
		m.filterOpen = ""
		m.filterSearch.Blur()
		return m, nil
	}
	before := m.filterSearch.Value()
	var cmd tea.Cmd
	m.filterSearch, cmd = m.filterSearch.Update(key)
	if m.filterSearch.Value() != before {
		// The query just narrowed (or widened) the list: clamp the cursor back onto it rather
		// than leaving it pointing past the end, or at an unrelated row, of the new list.
		if n := len(m.filterOptionIDs()) + 1; m.filterIdx >= n {
			m.filterIdx = n - 1
		}
		if m.filterIdx < 0 {
			m.filterIdx = 0
		}
	}
	return m, cmd
}

func (m Model) viewFilterPicker() string {
	title := "Filter by assignee"
	switch m.filterOpen {
	case "label":
		title = "Filter by label"
	case "state":
		title = "Filter by state"
	case "priority":
		title = "Filter by priority"
	}
	out := columnHeaderStyle.Render(title) + "\n"
	hint := "j/k  move    enter  apply    esc  cancel"
	if m.filterOpen == "label" {
		out += helpStyle.Render("Search: ") + m.filterSearch.View() + "\n"
		hint = "type  search    up/down  move    enter  apply    esc  cancel"
	}
	out += pickerLine("All", m.filterIdx == 0)
	for i, label := range m.filterOptionLabels() {
		out += pickerLine(label, m.filterIdx == i+1)
	}
	out += helpStyle.Render(hint)
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
	if m.filterState != "" {
		for _, st := range m.states {
			if st.ID == m.filterState {
				parts = append(parts, "state: "+st.Name)
				break
			}
		}
	}
	if m.filterPriority != "" {
		parts = append(parts, "priority: "+m.filterPriority)
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
