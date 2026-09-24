package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	m.resetPickerSearch()
	for i, st := range m.states {
		if st.ID == item.State {
			m.pickerIdx = i
			break
		}
	}
}

// resetPickerSearch clears and focuses the type-to-filter query box (see
// updateSearchablePicker/filteredStates/filteredLabels) — called whenever the state or labels
// picker opens, so it never starts out still filtered by whatever was typed the last time.
func (m *Model) resetPickerSearch() {
	m.pickerSearch.Reset()
	m.pickerSearch.Focus()
}

// filteredStates is m.states narrowed by the state picker's search query, case-insensitively
// matching the name. An empty query (the common case: arrow to the option, don't type anything)
// matches everything, so the picker looks and behaves exactly as it did before it could filter.
func (m Model) filteredStates() []api.State {
	q := strings.ToLower(strings.TrimSpace(m.pickerSearch.Value()))
	if q == "" {
		return m.states
	}
	out := make([]api.State, 0, len(m.states))
	for _, st := range m.states {
		if strings.Contains(strings.ToLower(st.Name), q) {
			out = append(out, st)
		}
	}
	return out
}

// filteredLabels is filteredStates for the labels picker.
func (m Model) filteredLabels() []api.Label {
	q := strings.ToLower(strings.TrimSpace(m.pickerSearch.Value()))
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

// assigneePickerRows is the ordered list of member IDs the assignee/new-item-assignee picker
// shows for the current search query, filteredStates/filteredLabels' equivalent for a picker
// that also has the synthetic leading "Unassigned" row: "" stands for that row, included only
// when it matches the query too, so typing narrows it away like any other option.
func (m Model) assigneePickerRows() []string {
	q := strings.ToLower(strings.TrimSpace(m.pickerSearch.Value()))
	out := make([]string, 0, len(m.members)+1)
	if q == "" || strings.Contains("unassigned", q) {
		out = append(out, "")
	}
	for _, mem := range m.members {
		if q == "" || strings.Contains(strings.ToLower(mem.Name()), q) {
			out = append(out, mem.ID)
		}
	}
	return out
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

// openAssigneePicker opens the project's member list — with a leading "Unassigned" entry —
// against the active item (the board's selected card, or the detail screen's open item). Like
// openStatePicker/openPriorityPicker it PATCHes the work item itself (see updatePicker's
// "assignee" case), unlike the board's own "a" key, which only filters what the board shows.
// A work item can carry several assignees, but this picker is a single choice — picking one
// replaces the whole list, the same way the new-item review step's assignee picker sets one.
func (m *Model) openAssigneePicker() {
	item := m.activeItem()
	if item == nil {
		return
	}
	m.pickerOpen = "assignee"
	m.pickerIdx = 0
	m.resetPickerSearch()
	if len(item.Assignees) == 0 {
		return
	}
	first := item.Assignees[0]
	for i, id := range m.assigneePickerRows() {
		if id == first {
			m.pickerIdx = i
			break
		}
	}
}

// openNewItemStatePicker/openNewItemPriorityPicker/openNewItemAssigneePicker are the state,
// priority and assignee pickers for the board's new-work-item review step
// (updateNewItemReview): unlike openStatePicker/openPriorityPicker, which act on
// m.activeItem() (an existing work item), these apply to the item still being composed —
// there's nothing yet to patch, so the choice just lands in m.newItem*.

func (m *Model) openNewItemStatePicker() {
	m.pickerOpen = "state"
	m.pickerIdx = 0
	m.resetPickerSearch()
	for i, st := range m.states {
		if st.ID == m.newItemStateID {
			m.pickerIdx = i
			break
		}
	}
}

func (m *Model) openNewItemPriorityPicker() {
	m.pickerOpen = "priority"
	m.pickerIdx = 0
	for i, p := range api.Priorities {
		if p == m.newItemPriority {
			m.pickerIdx = i
			break
		}
	}
}

// openNewItemAssigneePicker opens the project's member list with a leading "Unassigned"
// entry, the same shape the assignee filter picker uses (see filterOptionLabels).
func (m *Model) openNewItemAssigneePicker() {
	m.pickerOpen = "new-item-assignee"
	m.pickerIdx = 0
	m.resetPickerSearch()
	if m.newItemAssignee == "" {
		return
	}
	for i, id := range m.assigneePickerRows() {
		if id == m.newItemAssignee {
			m.pickerIdx = i
			break
		}
	}
}

// openLabelPicker opens the project's label list against the active item (the board's
// selected card, or the detail screen's open item) as a toggle-multiple-then-confirm picker:
// unlike openStatePicker/openPriorityPicker/openAssigneePicker, which pick exactly one entry,
// a work item can carry several labels at once, so this one tracks a working set in
// m.labelPickerSelected (seeded from the item's current labels) that space toggles and enter
// PATCHes as a whole.
func (m *Model) openLabelPicker() {
	item := m.activeItem()
	if item == nil {
		return
	}
	m.pickerOpen = "labels"
	m.pickerIdx = 0
	m.resetPickerSearch()
	m.labelPickerSelected = make(map[string]bool, len(item.Labels))
	for _, id := range item.Labels {
		m.labelPickerSelected[id] = true
	}
}

// openNewItemLabelPicker is openLabelPicker for the new-work-item review step: the working set
// starts from m.newItemLabels since there is no work item yet to read labels off of.
func (m *Model) openNewItemLabelPicker() {
	m.pickerOpen = "labels"
	m.pickerIdx = 0
	m.resetPickerSearch()
	m.labelPickerSelected = make(map[string]bool, len(m.newItemLabels))
	for _, id := range m.newItemLabels {
		m.labelPickerSelected[id] = true
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
		return len(m.filteredStates())
	case "sort":
		return len(sortModes)
	case "subissue":
		return len(m.subIssueOptions())
	case "assignee", "new-item-assignee":
		return len(m.assigneePickerRows())
	case "labels":
		return len(m.filteredLabels())
	case "links":
		return len(m.linkOptions)
	case "attachments":
		return len(m.attachments)
	case "color-target":
		return len(m.colorTargetRows())
	}
	return len(api.Priorities)
}

func (m Model) updatePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	// The state, labels and assignee pickers are type-to-filter search boxes (see
	// updateSearchablePicker): j/k/x are ordinary characters to type there, not shortcuts, so
	// they need their own key handling entirely separate from every other (plain list) picker
	// below.
	switch m.pickerOpen {
	case "state", "labels", "assignee", "new-item-assignee":
		return m.updateSearchablePicker(key)
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
	case "a":
		// Attach a new file — only meaningful while the attachments picker is open; "a" is
		// otherwise unused across the plain-list pickers this switch handles.
		if m.pickerOpen == "attachments" {
			m.pickerOpen = ""
			return m.openAttachPathPrompt()
		}
	case "enter":
		return m.applyPickerEnter()
	}
	return m, nil
}

// updateSearchablePicker drives the state/labels/assignee pickers' type-to-filter query box
// (m.pickerSearch): the up/down arrows move within the filtered list (see
// filteredStates/filteredLabels/assigneePickerRows), space toggles the highlighted label (labels
// only), enter applies/saves and esc cancels, exactly as the plain pickers' j/k/enter/esc do.
// Every other keystroke edits the query itself — including space on every picker but labels, so
// state/assignee search can still match a multi-word name.
func (m Model) updateSearchablePicker(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc":
		m.pickerOpen = ""
		m.pickerSearch.Blur()
		return m, nil
	case "up":
		if m.pickerIdx > 0 {
			m.pickerIdx--
		}
		return m, nil
	case "down":
		if m.pickerIdx < m.pickerOptionCount()-1 {
			m.pickerIdx++
		}
		return m, nil
	case " ":
		if m.pickerOpen == "labels" {
			opts := m.filteredLabels()
			if m.pickerIdx >= 0 && m.pickerIdx < len(opts) {
				id := opts[m.pickerIdx].ID
				m.labelPickerSelected[id] = !m.labelPickerSelected[id]
			}
			return m, nil
		}
	case "enter":
		return m.applyPickerEnter()
	}
	before := m.pickerSearch.Value()
	var cmd tea.Cmd
	m.pickerSearch, cmd = m.pickerSearch.Update(key)
	if m.pickerSearch.Value() != before {
		// The query just narrowed (or widened) the list: clamp the cursor back onto it rather
		// than leaving it pointing past the end, or at an unrelated row, of the new list.
		if n := m.pickerOptionCount(); m.pickerIdx >= n {
			m.pickerIdx = n - 1
		}
		if m.pickerIdx < 0 {
			m.pickerIdx = 0
		}
	}
	return m, cmd
}

// applyPickerEnter is every picker's enter key, shared by the plain list pickers (updatePicker)
// and the searchable ones (updateSearchablePicker) so both apply a selection exactly the same
// way regardless of how the user got there.
func (m Model) applyPickerEnter() (tea.Model, tea.Cmd) {
	m.pickerSearch.Blur()
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
	if m.pickerOpen == "links" {
		if m.pickerIdx < 0 || m.pickerIdx >= len(m.linkOptions) {
			m.pickerOpen = ""
			return m, nil
		}
		url := m.linkOptions[m.pickerIdx]
		m.pickerOpen = ""
		return m.openLinkInBrowser(url)
	}
	if m.pickerOpen == "attachments" {
		if m.pickerIdx < 0 || m.pickerIdx >= len(m.attachments) {
			m.pickerOpen = ""
			return m, nil
		}
		att := m.attachments[m.pickerIdx]
		m.pickerOpen = ""
		return m.downloadAttachmentCmd(att)
	}
	if m.pickerOpen == "color-target" {
		row, ok := m.colorTargetAt(m.pickerIdx)
		m.pickerOpen = ""
		if !ok {
			return m, nil
		}
		m.openColorPrompt(row.kind, row.id, row.hex)
		return m, nil
	}
	if m.creatingItem {
		// The item being composed does not exist yet, so there is nothing to PATCH: the
		// choice just lands in m.newItem* for the review step (viewNewItemReview) to show.
		switch m.pickerOpen {
		case "state":
			opts := m.filteredStates()
			if m.pickerIdx >= 0 && m.pickerIdx < len(opts) {
				m.newItemStateID = opts[m.pickerIdx].ID
			}
		case "priority":
			if m.pickerIdx >= 0 && m.pickerIdx < len(api.Priorities) {
				m.newItemPriority = api.Priorities[m.pickerIdx]
			}
		case "new-item-assignee":
			rows := m.assigneePickerRows()
			if m.pickerIdx >= 0 && m.pickerIdx < len(rows) {
				m.newItemAssignee = rows[m.pickerIdx]
			}
		case "labels":
			ids := []string{}
			for _, l := range m.labels {
				if m.labelPickerSelected[l.ID] {
					ids = append(ids, l.ID)
				}
			}
			m.newItemLabels = ids
		}
		m.pickerOpen = ""
		return m, nil
	}
	item := m.activeItem()
	if item == nil {
		m.pickerOpen = ""
		return m, nil
	}
	var patch map[string]any
	switch m.pickerOpen {
	case "state":
		opts := m.filteredStates()
		if m.pickerIdx < 0 || m.pickerIdx >= len(opts) {
			m.pickerOpen = ""
			return m, nil
		}
		patch = map[string]any{"state": opts[m.pickerIdx].ID}
		m.status = "Changing state..."
	case "assignee":
		// ids starts non-nil (rather than a nil slice) so clearing the assignee PATCHes
		// "assignees": [] — a nil slice marshals to JSON null, which is not the same
		// instruction to the API as an empty list.
		ids := []string{}
		rows := m.assigneePickerRows()
		if m.pickerIdx >= 0 && m.pickerIdx < len(rows) && rows[m.pickerIdx] != "" {
			ids = []string{rows[m.pickerIdx]}
		}
		patch = map[string]any{"assignees": ids}
		// Naming who it's being assigned to (rather than a generic "Updating...") is the
		// status message's one chance to tell this apart, in the moment, from the board's own
		// "a" key: that one only filters the board and never shows a status at all — pressing
		// the wrong one and seeing "Assigning to Bob..." here says immediately that this PATCHed
		// the card instead of filtering it, rather than leaving that to be inferred from the
		// board looking unfiltered afterwards.
		name := "Unassigned"
		if len(ids) > 0 {
			name = m.memberName(ids[0])
		}
		m.status = "Assigning to " + name + "..."
	case "labels":
		// ids starts non-nil for the same reason the assignee PATCH above does: a nil
		// slice marshals to JSON null, and clearing every label needs to PATCH "labels":
		// [] instead. The full label list (not the filtered one), since labelPickerSelected
		// tracks the working set across query changes rather than the filtered rows.
		ids := []string{}
		for _, l := range m.labels {
			if m.labelPickerSelected[l.ID] {
				ids = append(ids, l.ID)
			}
		}
		patch = map[string]any{"labels": ids}
		m.status = "Updating labels..."
	default:
		patch = map[string]any{"priority": api.Priorities[m.pickerIdx]}
		m.status = "Changing priority..."
	}
	return m, updateWorkItem(m.client, m.workspaceSlug, m.project.ID, item.ID, patch)
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
	case "assignee":
		title = "Change assignee"
	case "new-item-assignee":
		title = "New item assignee"
	case "labels":
		title = "Change labels"
	case "links":
		title = "Open link"
	case "attachments":
		title = "Attachments"
	case "color-target":
		title = "Recolor a state, label or priority"
	}
	if m.creatingItem {
		switch m.pickerOpen {
		case "state":
			title = "New item state"
		case "priority":
			title = "New item priority"
		case "labels":
			title = "New item labels"
		}
	}
	out := columnHeaderStyle.Render(title) + "\n"
	hint := "j/k  move    enter  apply    esc  cancel"
	switch {
	case m.pickerOpen == "state":
		out += helpStyle.Render("Search: ") + m.pickerSearch.View() + "\n"
		for i, st := range m.filteredStates() {
			out += pickerLine(st.Name, i == m.pickerIdx)
		}
		hint = "type  search    up/down  move    enter  apply    esc  cancel"
	case m.pickerOpen == "sort":
		for i, sm := range sortModes {
			out += pickerLine(sm.label, i == m.pickerIdx)
		}
	case m.pickerOpen == "assignee", m.pickerOpen == "new-item-assignee":
		out += helpStyle.Render("Search: ") + m.pickerSearch.View() + "\n"
		for i, id := range m.assigneePickerRows() {
			name := "Unassigned"
			if id != "" {
				name = m.memberName(id)
			}
			out += pickerLine(name, i == m.pickerIdx)
		}
		hint = "type  search    up/down  move    enter  apply    esc  cancel"
	case m.pickerOpen == "labels":
		out += helpStyle.Render("Search: ") + m.pickerSearch.View() + "\n"
		for i, l := range m.filteredLabels() {
			box := "[ ] "
			if m.labelPickerSelected[l.ID] {
				box = "[x] "
			}
			out += pickerLine(box+l.Name, i == m.pickerIdx)
		}
		hint = "type  search    up/down  move    space  toggle    enter  save    esc  cancel"
	case m.pickerOpen == "links":
		for i, u := range m.linkOptions {
			out += pickerLine(truncate(u, 70), i == m.pickerIdx)
		}
		hint = "j/k  move    enter  open    esc  cancel"
	case m.pickerOpen == "attachments":
		switch {
		case m.attachmentsLoading:
			out += helpStyle.Render("Loading attachments...") + "\n"
		case len(m.attachments) == 0:
			out += helpStyle.Render("(no attachments)") + "\n"
		default:
			for i, att := range m.attachments {
				line := att.Name() + "  " + helpStyle.Render(humanSize(att.Size()))
				out += pickerLine(line, i == m.pickerIdx)
			}
		}
		if m.attachUploading {
			out += helpStyle.Render("Uploading...") + "\n"
		}
		hint = "j/k  move    enter  download    a  attach new    esc  cancel"
	case m.pickerOpen == "color-target":
		for i, row := range m.colorTargetRows() {
			out += m.colorTargetLine(row.label, row.hex, row.pastel, i == m.pickerIdx)
		}
		hint = "j/k  move    enter  choose color    esc  cancel"
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

// colorTargetLine is pickerLine for the color-target picker: an unselected row is tinted in its
// own effective color (see effectiveStateColor/effectiveLabelColor) so the list doubles as a
// preview of what is about to be recolored, exactly like a board column header or a card's
// labels. The selected row falls back to plain pickerLine instead: cardSelectedStyle's own
// background/foreground must not be interrupted by a nested style's reset, the same reason
// cardLines avoids colored labels on a selected card.
func (m Model) colorTargetLine(label, hex string, pastel, selected bool) string {
	if selected {
		return pickerLine(label, true)
	}
	color := labelColor(hex)
	if pastel {
		color = pastelStateColor(hex)
	}
	return cardStyle.Render("  "+lipgloss.NewStyle().Foreground(color).Render(label)) + "\n"
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
