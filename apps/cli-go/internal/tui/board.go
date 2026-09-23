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
	m.focusedCol = 0
	m.colCursor = make([]int, len(m.states))
	m.screen = screenBoard
	return m, nil
}

// columnItems returns the work items in the given state, in a stable order.
func (m Model) columnItems(stateID string) []api.WorkItem {
	var out []api.WorkItem
	for _, it := range m.items {
		if it.State == stateID {
			out = append(out, it)
		}
	}
	return out
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
		m.screen = screenWorkspaceInput
		m.workspaceInput.Focus()
	case "enter":
		if item := m.selectedItem(); item != nil {
			m.detailItem = item
			m.screen = screenDetail
		}
	case "s":
		m.openStatePicker()
	case "y":
		m.openPriorityPicker()
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

	colWidth := 24
	if m.width > 0 {
		if w := m.width/len(m.states) - 4; w >= 12 {
			colWidth = w
		}
	}

	var cols []string
	for ci, st := range m.states {
		style := columnStyle
		if ci == m.focusedCol {
			style = columnFocusedStyle
		}
		items := m.columnItems(st.ID)
		header := columnHeaderStyle.Render(fmt.Sprintf("%s (%d)", st.Name, len(items)))
		body := header + "\n"
		for ii, it := range items {
			line := fmt.Sprintf("#%d %s", it.SequenceID, truncate(it.Name, colWidth-6))
			if ci == m.focusedCol && ii == m.colCursor[ci] {
				body += cardSelectedStyle.Width(colWidth).Render(line) + "\n"
			} else {
				body += cardStyle.Width(colWidth).Render(line) + "\n"
			}
		}
		cols = append(cols, style.Width(colWidth).Render(strings.TrimRight(body, "\n")))
	}

	out := titleStyle.Render(m.project.Name) + "\n\n"
	out += lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	out += "\n\n" + m.footer("h/l  column    j/k  card    enter  open    s  state    y  priority    r  refresh    p  boards    q  quit")
	if m.pickerOpen != "" {
		out += "\n\n" + m.viewPicker()
	}
	return out
}

func truncate(s string, n int) string {
	if n <= 1 || len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
