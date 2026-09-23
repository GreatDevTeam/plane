package tui

import tea "github.com/charmbracelet/bubbletea"

func (m Model) updateProjects(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.projectIdx > 0 {
			m.projectIdx--
		}
	case "down", "j":
		if m.projectIdx < len(m.projects)-1 {
			m.projectIdx++
		}
	case "w":
		return m.switchWorkspace()
	case "enter":
		if len(m.projects) == 0 {
			return m, nil
		}
		m.project = m.projects[m.projectIdx]
		m.loading = true
		m.status = "Loading board..."
		m.screen = screenBoard
		return m, tea.Batch(
			fetchBoard(m.client, m.workspaceSlug, m.project.ID),
			fetchBoardExtras(m.client, m.workspaceSlug, m.project.ID),
		)
	}
	return m, nil
}

func (m Model) viewProjects() string {
	out := titleStyle.Render("Boards in "+m.workspaceSlug) + "\n\n"
	if m.loading {
		out += "Loading...\n"
	} else if len(m.projects) == 0 {
		out += helpStyle.Render("No projects in this workspace.") + "\n"
	}
	for i, p := range m.projects {
		line := p.Name
		if p.Identifier != "" {
			line = p.Identifier + "  " + p.Name
		}
		if i == m.projectIdx {
			out += cardSelectedStyle.Render("> "+line) + "\n"
		} else {
			out += cardStyle.Render("  "+line) + "\n"
		}
	}
	out += "\n" + m.footer(helpStyle.Render("j/k  move    enter  open board    w  switch workspace    q  quit"))
	return out
}
