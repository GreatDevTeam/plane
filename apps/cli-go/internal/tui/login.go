package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/config"
)

func isTextInputScreen(s screen) bool {
	switch s {
	case screenServerInput, screenTokenInput, screenEmailInput, screenPasswordInput, screenWorkspaceInput:
		return true
	}
	return false
}

// --- server URL ---

func (m Model) updateServerInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			m.serverURL = strings.TrimRight(strings.TrimSpace(m.serverInput.Value()), "/")
			if m.serverURL == "" {
				m.setError(errRequired("server URL"))
				return m, nil
			}
			m.setError(nil)
			if m.serverURL == m.cfg.ServerURL && m.cfg.Token != "" {
				m.status = "Signing in with saved token..."
				m.loading = true
				return m, loginWithToken(m.serverURL, m.cfg.Token)
			}
			m.screen = screenAuthChoice
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.serverInput, cmd = m.serverInput.Update(msg)
	return m, cmd
}

func (m Model) viewServerInput() string {
	return titleStyle.Render("plane-cli") + "\n\n" +
		"Server URL:\n" + focusedInputStyle.Render(m.serverInput.View()) + "\n\n" +
		m.footer(helpStyle.Render("enter  continue    ctrl+c  quit"))
}

// --- auth method choice ---

var authChoices = []string{"API token", "Email / password"}

func (m Model) updateAuthChoice(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.authChoiceIdx > 0 {
				m.authChoiceIdx--
			}
		case "down", "j":
			if m.authChoiceIdx < len(authChoices)-1 {
				m.authChoiceIdx++
			}
		case "esc":
			m.screen = screenServerInput
		case "enter":
			m.setError(nil)
			if m.authChoiceIdx == 0 {
				m.screen = screenTokenInput
				m.tokenInput.Focus()
			} else {
				m.screen = screenEmailInput
				m.emailInput.Focus()
			}
		}
	}
	return m, nil
}

func (m Model) viewAuthChoice() string {
	out := titleStyle.Render("Sign in to "+m.serverURL) + "\n\n"
	for i, c := range authChoices {
		if i == m.authChoiceIdx {
			out += cardSelectedStyle.Render("> "+c) + "\n"
		} else {
			out += cardStyle.Render("  "+c) + "\n"
		}
	}
	out += "\n" + m.footer(helpStyle.Render("j/k  move    enter  select    esc  back"))
	return out
}

// --- API token ---

func (m Model) updateTokenInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.screen = screenAuthChoice
			return m, nil
		case "enter":
			token := strings.TrimSpace(m.tokenInput.Value())
			if token == "" {
				m.setError(errRequired("API token"))
				return m, nil
			}
			m.setError(nil)
			m.status = "Signing in..."
			m.loading = true
			return m, loginWithToken(m.serverURL, token)
		}
	}
	var cmd tea.Cmd
	m.tokenInput, cmd = m.tokenInput.Update(msg)
	return m, cmd
}

func (m Model) viewTokenInput() string {
	return titleStyle.Render("Sign in to "+m.serverURL) + "\n\n" +
		"API token (Settings -> API tokens in Plane):\n" + focusedInputStyle.Render(m.tokenInput.View()) + "\n\n" +
		m.footer(helpStyle.Render("enter  sign in    esc  back"))
}

// --- email / password ---

func (m Model) updateEmailInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.screen = screenAuthChoice
			return m, nil
		case "enter":
			email := strings.TrimSpace(m.emailInput.Value())
			if email == "" {
				m.setError(errRequired("email"))
				return m, nil
			}
			m.setError(nil)
			m.pendingEmail = email
			m.screen = screenPasswordInput
			m.passwordInput.SetValue("")
			m.passwordInput.Focus()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.emailInput, cmd = m.emailInput.Update(msg)
	return m, cmd
}

func (m Model) viewEmailInput() string {
	return titleStyle.Render("Sign in to "+m.serverURL) + "\n\n" +
		"Email:\n" + focusedInputStyle.Render(m.emailInput.View()) + "\n\n" +
		m.footer(helpStyle.Render("enter  continue    esc  back"))
}

func (m Model) updatePasswordInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.screen = screenEmailInput
			return m, nil
		case "enter":
			password := m.passwordInput.Value()
			if password == "" {
				m.setError(errRequired("password"))
				return m, nil
			}
			m.setError(nil)
			m.status = "Signing in..."
			m.loading = true
			return m, loginWithPassword(m.serverURL, m.pendingEmail, password)
		}
	}
	var cmd tea.Cmd
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m Model) viewPasswordInput() string {
	return titleStyle.Render("Sign in to "+m.serverURL) + "\n\n" +
		"Password for " + m.pendingEmail + ":\n" + focusedInputStyle.Render(m.passwordInput.View()) + "\n\n" +
		m.footer(helpStyle.Render("enter  sign in    esc  back"))
}

// --- shared login result handling ---

func (m Model) handleLoginResult(msg loginResultMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.status = ""
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	m.client = msg.client
	m.user = msg.user
	m.cfg.ServerURL = m.serverURL
	m.cfg.Token = msg.token
	m.workspaces = msg.workspaces
	_ = config.Save(m.cfg)

	if m.cfg.WorkspaceSlug != "" {
		m.workspaceSlug = m.cfg.WorkspaceSlug
		m.workspaceInput.SetValue(m.cfg.WorkspaceSlug)
		m.loading = true
		m.status = "Loading projects..."
		m.screen = screenProjects
		return m, fetchProjects(m.client, m.workspaceSlug)
	}

	// No cached slug yet: if sign-in gave us the workspace list (only possible right after
	// an email/password login — see auth.PasswordLogin), detect it instead of asking.
	switch len(m.workspaces) {
	case 0:
		m.screen = screenWorkspaceInput
		m.workspaceInput.Focus()
		return m, nil
	case 1:
		return m.selectWorkspace(m.workspaces[0])
	default:
		m.workspacePicks = 0
		m.screen = screenWorkspacePicker
		return m, nil
	}
}

func errRequired(field string) error {
	return &requiredFieldError{field}
}

type requiredFieldError struct{ field string }

func (e *requiredFieldError) Error() string { return e.field + " is required" }
