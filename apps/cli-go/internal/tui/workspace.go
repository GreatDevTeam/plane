package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
	"github.com/makeplane/plane/apps/cli-go/internal/config"
)

// selectWorkspace commits to a detected workspace (skipping the manual slug prompt
// entirely) and starts loading its projects.
func (m Model) selectWorkspace(ws api.Workspace) (tea.Model, tea.Cmd) {
	slug := ws.Slug
	m.setError(nil)
	m.workspaceSlug = slug
	m.workspaceInput.SetValue(slug)
	m.cfg.WorkspaceSlug = slug
	_ = config.Save(m.cfg)
	m.loading = true
	m.status = "Loading projects..."
	m.screen = screenProjects
	return m, fetchProjects(m.client, slug)
}

// switchWorkspace is bound to "w" on the board and project screens. It reopens the
// workspace picker if a detected list is available (see auth.PasswordLogin), otherwise
// falls back to the manual slug prompt the same as a bare API token login would.
func (m Model) switchWorkspace() (tea.Model, tea.Cmd) {
	if len(m.workspaces) > 0 {
		m.workspacePicks = 0
		for i, ws := range m.workspaces {
			if ws.Slug == m.workspaceSlug {
				m.workspacePicks = i
				break
			}
		}
		m.screen = screenWorkspacePicker
		return m, nil
	}
	m.screen = screenWorkspaceInput
	m.workspaceInput.Focus()
	return m, nil
}

func (m Model) updateWorkspacePicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.workspacePicks > 0 {
			m.workspacePicks--
		}
	case "down", "j":
		if m.workspacePicks < len(m.workspaces)-1 {
			m.workspacePicks++
		}
	case "esc":
		m.screen = screenWorkspaceInput
		m.workspaceInput.Focus()
	case "enter":
		if len(m.workspaces) == 0 {
			return m, nil
		}
		return m.selectWorkspace(m.workspaces[m.workspacePicks])
	}
	return m, nil
}

func (m Model) viewWorkspacePicker() string {
	who := ""
	if m.user != nil {
		who = "Signed in as " + fmtUser(m.user) + "\n\n"
	}
	out := titleStyle.Render("Choose a workspace") + "\n\n" + who
	for i, ws := range m.workspaces {
		line := ws.Name + "  (" + ws.Slug + ")"
		if i == m.workspacePicks {
			out += cardSelectedStyle.Render("> "+line) + "\n"
		} else {
			out += cardStyle.Render("  "+line) + "\n"
		}
	}
	out += "\n" + m.footer("j/k  move    enter  select    esc  type slug instead    q  quit")
	return out
}

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
