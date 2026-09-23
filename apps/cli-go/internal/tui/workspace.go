package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/config"
)

func (m Model) updateWorkspaceInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			slug := strings.TrimSpace(m.workspaceInput.Value())
			if slug == "" {
				m.setError(errRequired("workspace slug"))
				return m, nil
			}
			m.setError(nil)
			m.workspaceSlug = slug
			m.loading = true
			m.status = "Loading projects..."
			return m, fetchProjects(m.client, slug)
		}
	}
	var cmd tea.Cmd
	m.workspaceInput, cmd = m.workspaceInput.Update(msg)
	return m, cmd
}

func (m Model) viewWorkspaceInput() string {
	who := ""
	if m.user != nil {
		who = "Signed in as " + fmtUser(m.user) + "\n\n"
	}
	return titleStyle.Render("Workspace") + "\n\n" + who +
		"Workspace slug (from the URL, e.g. app.plane.so/<slug>):\n" +
		focusedInputStyle.Render(m.workspaceInput.View()) + "\n\n" +
		m.footer("enter  continue    ctrl+c  quit")
}

func (m Model) handleProjects(msg projectsMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.status = ""
	if msg.err != nil {
		m.setError(msg.err)
		m.screen = screenWorkspaceInput
		m.workspaceInput.Focus()
		return m, nil
	}
	m.setError(nil)
	m.projects = msg.projects
	m.projectIdx = 0
	m.cfg.WorkspaceSlug = m.workspaceSlug
	_ = config.Save(m.cfg)
	m.screen = screenProjects
	return m, nil
}
