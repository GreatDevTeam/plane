package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

func (m Model) handleBoardData(msg boardDataMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.status = ""
	if msg.err != nil {
		m.setError(msg.err)
		m.screen = screenProjects
		return m, nil
	}
	m.setError(nil)
	m.states = msg.states
	m.items = msg.items
	m.itemsNextCursor = msg.nextCursor
	m.labels = msg.labels
	m.members = msg.members
	m.focusedCol = 0
	m.colCursor = make([]int, len(m.states))
	m.filterAssignee = ""
	m.filterLabel = ""
	m.screen = screenBoard
	if msg.hasNextPage {
		m.itemsLoadingMore = true
		return m, fetchWorkItemsPage(m.client, m.workspaceSlug, m.project.ID, msg.nextCursor)
	}
	return m, nil
}

// handleBoardItemsPage appends a streamed-in page of work items to the board that is
// already on screen, and immediately requests the next page if there is one. This is what
// lets a large project's board fill in progressively instead of blocking on every page
// before showing anything (and timing out entirely for very large projects).
func (m Model) handleBoardItemsPage(msg boardItemsPageMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.itemsLoadingMore = false
		m.setError(msg.err)
		return m, nil
	}
	m.items = append(m.items, msg.items...)
	m.itemsNextCursor = msg.nextCursor
	if msg.hasNext {
		return m, fetchWorkItemsPage(m.client, m.workspaceSlug, m.project.ID, msg.nextCursor)
	}
	m.itemsLoadingMore = false
	return m, nil
}

// columnItems returns the work items in the given state that also pass the active
// assignee/label filters (see filter.go), in a stable order.
func (m Model) columnItems(stateID string) []api.WorkItem {
	var out []api.WorkItem
	for _, it := range m.items {
		if it.State != stateID {
			continue
		}
		if m.filterAssignee != "" && !containsStr(it.Assignees, m.filterAssignee) {
			continue
		}
		if m.filterLabel != "" && !containsStr(it.Labels, m.filterLabel) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func (m Model) selectedItem() *api.WorkItem {
	if len(m.states) == 0 || m.focusedCol >= len(m.states) {
		return nil
	}
	col := m.columnItems(m.states[m.focusedCol].ID)
	if len(col) == 0 {
		return nil
	}
	idx := m.colCursor[m.focusedCol]
	if idx < 0 || idx >= len(col) {
		return nil
	}
	return &col[idx]
}

func (m Model) findState(id string) *api.State {
	for i := range m.states {
		if m.states[i].ID == id {
			return &m.states[i]
		}
	}
	return nil
}

func (m Model) updateBoard(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.pickerOpen != "" {
		return m.updatePicker(msg)
	}
	if m.filterOpen != "" {
		return m.updateFilterPicker(msg)
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q":
		return m, tea.Quit
	case "left", "h":
		if m.focusedCol > 0 {
			m.focusedCol--
		}
	case "right", "l":
		if m.focusedCol < len(m.states)-1 {
			m.focusedCol++
		}
	case "up", "k":
		if m.colCursor[m.focusedCol] > 0 {
			m.colCursor[m.focusedCol]--
		}
	case "down", "j":
		col := m.columnItems(m.states[m.focusedCol].ID)
		if m.colCursor[m.focusedCol] < len(col)-1 {
			m.colCursor[m.focusedCol]++
		}
	case "r":
		m.loading = true
		m.status = "Refreshing..."
		return m, fetchBoard(m.client, m.workspaceSlug, m.project.ID)
	case "p":
		m.screen = screenProjects
	case "w":
		return m.switchWorkspace()
	case "enter":
		if item := m.selectedItem(); item != nil {
			m.detailItem = item
			m.comments = nil
			m.commentCursor = 0
			m.commentsLoading = true
			m.screen = screenDetail
			return m, fetchComments(m.client, m.workspaceSlug, m.project.ID, item.ID)
		}
	case "s":
		m.openStatePicker()
	case "y":
		m.openPriorityPicker()
	case "a":
		m.openFilterPicker("assignee")
	case "L":
		m.openFilterPicker("label")
	}
	return m, nil
}

func (m Model) viewBoard() string {
	if m.loading {
		return titleStyle.Render(m.project.Name) + "\n\nLoading board...\n\n" + m.footer("")
	}
	if len(m.states) == 0 {
		return titleStyle.Render(m.project.Name) + "\n\n" + helpStyle.Render("This project has no states.") + "\n\n" + m.footer("p  switch board    q  quit")
	}

	const minColWidth = 20
	const maxColWidth = 40

	// Narrow terminal (or many columns): showing every column side by side would squeeze
	// each below a usable width, so show only the focused column, full width, instead.
	narrow := m.width > 0 && m.width/len(m.states) < minColWidth

	header := titleStyle.Render(m.project.Name)
	if f := m.activeFilterSummary(); f != "" {
		header += "  " + helpStyle.Render(f)
	}
	if m.itemsLoadingMore {
		header += "  " + helpStyle.Render(fmt.Sprintf("loading more… (%d so far)", len(m.items)))
	}
	header += "\n\n"
	var body string
	if narrow {
		body = m.renderColumn(m.focusedCol, minColWidth, true)
		body += "\n" + helpStyle.Render(fmt.Sprintf("column %d/%d", m.focusedCol+1, len(m.states)))
	} else {
		colWidth := maxColWidth
		if w := m.width/len(m.states) - 4; w >= minColWidth && w < colWidth {
			colWidth = w
		}
		var cols []string
		for ci := range m.states {
			cols = append(cols, m.renderColumn(ci, colWidth, false))
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	}

	out := header + body
	out += "\n\n" + m.footer("h/l  column    j/k  card    enter  open    s  state    y  priority    a  assignee    L  label    r  refresh    p  boards    q  quit")
	if m.pickerOpen != "" {
		out += "\n\n" + m.viewPicker()
	}
	if m.filterOpen != "" {
		out += "\n\n" + m.viewFilterPicker()
	}
	return out
}

// defaultTerminalHeight is the row-clipping fallback used before the first WindowSizeMsg
// has arrived (or if the terminal never reports one at all). Without it, a board rendered
// before m.height is known skips clipping entirely: on a project with a big list of tasks
// this produces a frame thousands of lines tall that no terminal, full screen or otherwise,
// can actually show.
const defaultTerminalHeight = 24

// renderColumn renders a single kanban column, clipping its card list to the terminal
// height (minus room for the title/header/footer) so a long column's own header always
// stays visible instead of being pushed off-screen.
func (m Model) renderColumn(ci, colWidth int, fullWidth bool) string {
	st := m.states[ci]
	style := columnStyle
	if ci == m.focusedCol {
		style = columnFocusedStyle
	}
	if fullWidth && m.width > 0 {
		colWidth = m.width - 4
	}

	items := m.columnItems(st.ID)
	cursor := m.colCursor[ci]
	visible := items
	scrolled := false

	height := m.height
	if height <= 0 {
		height = defaultTerminalHeight
	}
	maxRows := height - 8
	if maxRows < 3 {
		maxRows = 3
	}
	if len(items) > maxRows {
		start := cursor - maxRows/2
		if start < 0 {
			start = 0
		}
		if start+maxRows > len(items) {
			start = len(items) - maxRows
		}
		visible = items[start : start+maxRows]
		cursor -= start
		scrolled = true
	}

	headerText := fmt.Sprintf("%s (%d)", st.Name, len(items))
	if scrolled {
		headerText += " *"
	}
	out := columnHeaderStyle.Render(headerText) + "\n"
	for ii, it := range visible {
		line := fmt.Sprintf("#%d %s", it.SequenceID, truncate(it.Name, colWidth-6))
		if ci == m.focusedCol && ii == cursor {
			out += cardSelectedStyle.Width(colWidth).Render(line) + "\n"
		} else {
			out += cardStyle.Width(colWidth).Render(line) + "\n"
		}
	}
	return style.Width(colWidth).Render(strings.TrimRight(out, "\n"))
}

func truncate(s string, n int) string {
	if n <= 1 || len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
