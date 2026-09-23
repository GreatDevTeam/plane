// Package tui is the terminal UI for plane-cli: login, board (project) switching, a kanban
// board of work items grouped by state, and a work item detail/edit view.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
	"github.com/makeplane/plane/apps/cli-go/internal/config"
)

type screen int

const (
	screenServerInput screen = iota
	screenAuthChoice
	screenTokenInput
	screenEmailInput
	screenPasswordInput
	screenWorkspaceInput
	screenWorkspacePicker
	screenProjects
	screenBoard
	screenDetail
)

// Model is the single bubbletea model driving every screen of plane-cli.
type Model struct {
	screen screen
	width  int
	height int

	err    string
	status string

	showHelp bool

	// Inputs
	serverInput    textinput.Model
	tokenInput     textinput.Model
	emailInput     textinput.Model
	passwordInput  textinput.Model
	workspaceInput textinput.Model
	authChoiceIdx  int

	cfg           config.Config
	client        *api.Client
	user          *api.User
	workspaceSlug string
	serverURL     string
	pendingEmail  string

	// workspaces is the list fetched right after an email/password sign-in (see
	// auth.PasswordLogin); a bare API token has no session to fetch it with, so this stays
	// nil in that case and the user is asked for the slug directly instead.
	workspaces     []api.Workspace
	workspacePicks int

	projects   []api.Project
	projectIdx int
	loading    bool

	project          api.Project
	states           []api.State
	items            []api.WorkItem
	itemsNextCursor  string
	itemsLoadingMore bool
	labels           []api.Label
	members          []api.Member
	focusedCol       int
	colCursor        []int
	detailItem       *api.WorkItem

	pickerOpen string // "" | "state" | "priority"
	pickerIdx  int

	filterOpen     string // "" | "assignee" | "label"
	filterIdx      int
	filterAssignee string // member ID, "" = no filter
	filterLabel    string // label ID, "" = no filter

	comments        []api.Comment
	commentsLoading bool
	commentCursor   int

	commentEditor    textarea.Model
	commentEditorOn  bool
	editingCommentID string // "" while composing a new comment, set while editing an existing one
}

// New builds the initial model. If cfg has a saved server+token, the model starts by
// validating that token instead of asking the user to type the server URL again.
func New(cfg config.Config) Model {
	m := Model{
		cfg:            cfg,
		serverInput:    newInput("https://app.plane.so", 60),
		tokenInput:     newInput("plane_api_...", 60),
		emailInput:     newInput("you@example.com", 60),
		passwordInput:  newInput("", 60),
		workspaceInput: newInput("my-workspace", 60),
	}
	m.passwordInput.EchoMode = textinput.EchoPassword
	m.passwordInput.EchoCharacter = '*'

	m.commentEditor = textarea.New()
	m.commentEditor.Placeholder = "Write a comment..."
	m.commentEditor.ShowLineNumbers = false
	m.commentEditor.CharLimit = 0
	m.commentEditor.SetWidth(70)
	m.commentEditor.SetHeight(5)

	if cfg.ServerURL != "" && cfg.Token != "" {
		m.screen = screenServerInput
		m.serverInput.SetValue(cfg.ServerURL)
		m.tokenInput.SetValue(cfg.Token)
		m.serverURL = cfg.ServerURL
	} else {
		m.screen = screenServerInput
	}
	m.workspaceInput.SetValue(cfg.WorkspaceSlug)
	m.serverInput.Focus()
	return m
}

func newInput(placeholder string, width int) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Width = width
	ti.CharLimit = 256
	return ti
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		if msg.String() == "?" && !isTextInputScreen(m.screen) && !m.commentEditorOn {
			m.showHelp = true
			return m, nil
		}

	case loginResultMsg:
		return m.handleLoginResult(msg)
	case projectsMsg:
		return m.handleProjects(msg)
	case boardDataMsg:
		return m.handleBoardData(msg)
	case boardItemsPageMsg:
		return m.handleBoardItemsPage(msg)
	case workItemUpdatedMsg:
		return m.handleWorkItemUpdated(msg)
	case commentsMsg:
		return m.handleComments(msg)
	case commentSavedMsg:
		return m.handleCommentSaved(msg)
	}

	switch m.screen {
	case screenServerInput:
		return m.updateServerInput(msg)
	case screenAuthChoice:
		return m.updateAuthChoice(msg)
	case screenTokenInput:
		return m.updateTokenInput(msg)
	case screenEmailInput:
		return m.updateEmailInput(msg)
	case screenPasswordInput:
		return m.updatePasswordInput(msg)
	case screenWorkspaceInput:
		return m.updateWorkspaceInput(msg)
	case screenWorkspacePicker:
		return m.updateWorkspacePicker(msg)
	case screenProjects:
		return m.updateProjects(msg)
	case screenBoard:
		return m.updateBoard(msg)
	case screenDetail:
		return m.updateDetail(msg)
	}
	return m, nil
}

func (m Model) View() string {
	var body string
	switch m.screen {
	case screenServerInput:
		body = m.viewServerInput()
	case screenAuthChoice:
		body = m.viewAuthChoice()
	case screenTokenInput:
		body = m.viewTokenInput()
	case screenEmailInput:
		body = m.viewEmailInput()
	case screenPasswordInput:
		body = m.viewPasswordInput()
	case screenWorkspaceInput:
		body = m.viewWorkspaceInput()
	case screenWorkspacePicker:
		body = m.viewWorkspacePicker()
	case screenProjects:
		body = m.viewProjects()
	case screenBoard:
		body = m.viewBoard()
	case screenDetail:
		body = m.viewDetail()
	}
	if m.showHelp {
		return body + "\n\n" + m.viewHelp()
	}
	return body
}

func (m *Model) setError(err error) {
	if err != nil {
		m.err = err.Error()
	} else {
		m.err = ""
	}
}

func (m Model) footer(hint string) string {
	out := helpStyle.Render(hint)
	if m.err != "" {
		out += "\n" + errorStyle.Render("Error: "+m.err)
	}
	if m.status != "" {
		out += "\n" + helpStyle.Render(m.status)
	}
	return out
}

// helpSection is a titled group of shortcut lines in the help overlay.
type helpSection struct {
	heading string
	lines   [][2]string // [keys, description]
}

var helpSections = []helpSection{
	{"Global", [][2]string{
		{"?", "toggle this help"},
		{"ctrl+c", "quit"},
	}},
	{"Text field", [][2]string{
		{"enter", "confirm"},
		{"esc", "back"},
	}},
	{"Workspace picker", [][2]string{
		{"j/k", "move"},
		{"enter", "select"},
		{"esc", "type slug instead"},
	}},
	{"Projects", [][2]string{
		{"j/k or up/down", "move"},
		{"enter", "open board"},
		{"q", "quit"},
	}},
	{"Board", [][2]string{
		{"h/l or left/right", "switch column"},
		{"j/k or up/down", "move card"},
		{"enter", "open item"},
		{"s", "change state"},
		{"y", "change priority"},
		{"a", "filter by assignee"},
		{"L", "filter by label"},
		{"p", "switch board (project)"},
		{"r", "refresh"},
		{"q", "quit"},
	}},
	{"Detail", [][2]string{
		{"s", "change state"},
		{"y", "change priority"},
		{"j/k or up/down", "select comment"},
		{"c", "add comment"},
		{"e", "edit selected comment (own only)"},
		{"esc/backspace", "back"},
	}},
	{"Comment editor", [][2]string{
		{"ctrl+s", "save"},
		{"esc", "cancel"},
	}},
	{"Picker", [][2]string{
		{"j/k", "move"},
		{"enter", "apply"},
		{"esc", "cancel"},
	}},
}

func (m Model) viewHelp() string {
	body := titleStyle.Render("Shortcuts") + "\n\n"
	for _, sec := range helpSections {
		body += helpSectionStyle.Render(sec.heading+":") + "\n"
		for _, kv := range sec.lines {
			body += "  " + helpKeyStyle.Render(padRight(kv[0], 20)) + "  " + kv[1] + "\n"
		}
		body += "\n"
	}
	return focusedInputStyle.Render(strings.TrimRight(body, "\n"))
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func fmtUser(u *api.User) string {
	if u == nil {
		return ""
	}
	name := u.DisplayName
	if name == "" {
		name = fmt.Sprintf("%s %s", u.FirstName, u.LastName)
	}
	if name == "" {
		name = u.Email
	}
	return name
}
