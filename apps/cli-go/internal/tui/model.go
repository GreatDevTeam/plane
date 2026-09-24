// Package tui is the terminal UI for plane-cli: login, board (project) switching, a kanban
// board of work items grouped by state, and a work item detail/edit view.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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

	// extrasCache holds the last labels/members fetched for each project (fetchBoardExtras),
	// keyed by project ID, so switching back to a project already visited this session shows
	// them straight away instead of the picker going empty while a fresh copy loads — see
	// applyBoardExtras and the task that asked for it. Entries are refreshed at most every
	// extrasCacheTTL rather than on every board load/tick.
	extrasCache map[string]extrasCacheEntry

	// relatedItems caches work items fetched by ID because they are referenced by one on
	// screen but are missing from `items` — currently only the parent of an opened card
	// (archived, or on a page that has not arrived yet). See relations.go.
	relatedItems map[string]api.WorkItem

	// subCounts is how many sub-tasks each work item has, rebuilt once per board frame by
	// viewBoard so that rendering a card does not rescan the whole project. nil outside a
	// board render, where subIssueCount falls back to scanning.
	subCounts map[string]int

	// hiddenStates holds the state IDs whose columns are collapsed to a narrow placeholder
	// on the current project's board. It is loaded from (and saved back to) the config file,
	// so a board opens with the same columns collapsed as when it was last closed.
	hiddenStates map[string]bool

	// sortMode orders the cards inside every column (see sortModes in board.go).
	sortMode string

	// refreshing is true while the background 30s board refresh is in flight; autoRefreshOn
	// guards against starting a second tick chain when another board is opened.
	refreshing    bool
	autoRefreshOn bool

	// detailLoading is true while the open work item is being re-fetched in the background
	// (the detail screen renders the cached copy meanwhile).
	detailLoading bool

	// pickerOpen: "" | "state" | "priority" | "sort" | "subissue" | "assignee" | "labels" |
	// "links" | "attachments" | "color-target"
	pickerOpen string
	pickerIdx  int

	// labelPickerSelected holds the working set of toggled-on label IDs while pickerOpen ==
	// "labels" (openLabelPicker/updatePicker's "labels" case) — unlike every other picker,
	// this one is multi-choice, so a single pickerIdx cannot also carry the selection.
	labelPickerSelected map[string]bool

	// pickerSearch is the type-to-filter query box shown while pickerOpen is "state" or
	// "labels" (see updateSearchablePicker/filteredStates/filteredLabels).
	pickerSearch textinput.Model

	// linkOptions holds the links extracted from the focused detail pane while
	// pickerOpen == "links" (see openFocusedLinks).
	linkOptions []string

	// attachments/attachmentsLoading back the detail screen's "f" attachments picker (see
	// openAttachmentsPicker in attachments.go). Unlike comments there is no 30s auto-refresh
	// keeping this current — it is (re-)fetched every time the picker opens.
	attachments        []api.Attachment
	attachmentsLoading bool

	// attachPathPromptOpen/attachPathInput/attachUploading drive the "attach a new file"
	// prompt (the attachments picker's "a" — see openAttachPathPrompt in attachments.go): a
	// bare local path input, since plane-cli has no file browser of its own.
	attachPathPromptOpen bool
	attachPathInput      textinput.Model
	attachUploading      bool

	// colorPromptOpen/colorInput/colorTargetKind/colorTargetID drive the local color-override
	// prompt (board's "C", via pickerOpen == "color-target" — see openColorPrompt in
	// colors.go). This is a display preference only, saved to config.Config; it never changes
	// a state's or label's actual color in Plane.
	colorPromptOpen bool
	colorInput      textinput.Model
	colorTargetKind string // "state" | "label" | "priority"
	colorTargetID   string

	filterOpen     string // "" | "assignee" | "label" | "state" | "priority"
	filterIdx      int
	filterAssignee string // member ID, "" = no filter
	filterLabel    string // label ID, "" = no filter
	filterState    string // state ID, "" = no filter
	filterPriority string // priority value, "" = no filter

	comments        []api.Comment
	commentsLoading bool
	commentCursor   int

	// descViewport and commentsViewport are the detail screen's two independently
	// scrollable panes (see detailPaneRows/detailLayout in detail.go). detailFocus says
	// which one j/k currently drives: within detailPaneComments that still moves
	// commentCursor, exactly as before this pair of viewports existed.
	descViewport     viewport.Model
	commentsViewport viewport.Model
	detailFocus      detailPane

	editor           textarea.Model
	editorOn         bool
	editorMode       string // "comment" | "description" | "new-item", meaningful while editorOn
	editingCommentID string // "" while composing a new comment, set while editing an existing one

	// New work item creation (board's "n"): the title is typed in the shared editor
	// (editorMode == "new-item"), then creatingItem drives a review step — reusing the
	// state/priority pickers plus a dedicated assignee one — where the user can change where
	// it lands before the POST actually fires. See openNewItemEditor/updateNewItemReview.
	creatingItem    bool
	newItemName     string
	newItemStateID  string
	newItemPriority string
	newItemAssignee string   // member ID, "" = unassigned
	newItemLabels   []string // label IDs to create the item with

	// idPromptOpen/idInput drive the board's "open by work item id" prompt (g): a bare
	// numeric input, looked up against the board's own m.items (see updateIDPrompt).
	idPromptOpen bool
	idInput      textinput.Model
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

	m.editor = textarea.New()
	m.editor.Placeholder = "Write a comment..."
	m.editor.ShowLineNumbers = false
	m.editor.CharLimit = 0
	m.editor.SetWidth(70)
	m.editor.SetHeight(5)
	m.sortMode = normalizeSortMode(cfg.SortMode)
	m.hiddenStates = make(map[string]bool)
	m.idInput = newInput("e.g. 123", 12)
	m.pickerSearch = newInput("type to search...", 40)
	m.attachPathInput = newInput("/path/to/file", 60)
	m.colorInput = newInput("ff8800", 10)
	m.extrasCache = make(map[string]extrasCacheEntry)

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

// Update dispatches msg to the real update logic, then works around a bubbletea rendering
// gap on terminals that never report a size (m.width/m.height stay 0 — see
// defaultTerminalWidth/defaultTerminalHeight in board.go): bubbletea's own renderer only
// erases the stale tail of a line, or drops now-unused trailing lines, when it knows the
// terminal's width (charmbracelet/bubbletea standard_renderer.go flush(), the
// `if r.width > 0` guard around EraseLineRight). With width unknown that guard never fires,
// so a board frame that shrinks between renders (a filter narrows the list, a column comes
// into focus with fewer cards, …) leaves the wider previous frame's leftover characters on
// screen — producing exactly the split, overlapping text a real report showed. Forcing a
// full ClearScreen on every update while the board is on screen with an unknown size sidesteps
// that guard entirely (ClearScreen writes ansi.EraseEntireScreen, which doesn't depend on a
// known width). This only affects the degraded no-size case; a terminal that reports its size
// normally never takes this path.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, cmd := m.update(msg)
	if nm, ok := newModel.(Model); ok && nm.screen == screenBoard && (nm.width <= 0 || nm.height <= 0) {
		cmd = tea.Batch(cmd, tea.ClearScreen)
	}
	return newModel, cmd
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		if msg.String() == "?" && !isTextInputScreen(m.screen) && !m.editorOn {
			m.showHelp = true
			return m, nil
		}

	case loginResultMsg:
		return m.handleLoginResult(msg)
	case projectsMsg:
		return m.handleProjects(msg)
	case boardDataMsg:
		return m.handleBoardData(msg)
	case boardExtrasMsg:
		return m.handleBoardExtras(msg)
	case boardItemsPageMsg:
		return m.handleBoardItemsPage(msg)
	case boardRefreshedMsg:
		return m.handleBoardRefreshed(msg)
	case boardTickMsg:
		return m.handleBoardTick(msg)
	case workItemLoadedMsg:
		return m.handleWorkItemLoaded(msg)
	case workItemUpdatedMsg:
		return m.handleWorkItemUpdated(msg)
	case commentsMsg:
		return m.handleComments(msg)
	case commentSavedMsg:
		return m.handleCommentSaved(msg)
	case workItemCreatedMsg:
		return m.handleWorkItemCreated(msg)
	case relatedWorkItemMsg:
		return m.handleRelatedWorkItem(msg)
	case attachmentsMsg:
		return m.handleAttachments(msg)
	case attachmentDownloadedMsg:
		return m.handleAttachmentDownloaded(msg)
	case attachmentUploadedMsg:
		return m.handleAttachmentUploaded(msg)
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

// footer renders the screen's bottom area: hint, then an error line, then a status line —
// each only if set. hint is taken as already styled (packHints and every literal hint string
// passed in wrap themselves in helpStyle/helpKeyStyle) rather than wrapped here, which is what
// lets a hint line color its shortcuts (helpKeyStyle) differently from the text describing
// them (helpStyle) — wrapping the whole thing in one style here, as this used to, would have
// made that impossible.
func (m Model) footer(hint string) string {
	out := hint
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
		{"A", "change assignee"},
		{"a", "filter by assignee"},
		{"L", "filter by label"},
		{"u", "copy work item url"},
		{"g", "open by work item id"},
		{"o", "card order"},
		{"n", "new work item"},
		{"x", "hide/show this column"},
		{"C", "recolor a state/label/priority (local only)"},
		{"p", "switch board (project)"},
		{"r", "refresh"},
		{"q", "quit"},
	}},
	{"Detail", [][2]string{
		{"s", "change state"},
		{"y", "change priority"},
		{"A", "change assignee"},
		{"d", "edit description"},
		{"g", "go to parent work item"},
		{"S", "jump to a sub-task"},
		{"u", "copy work item url"},
		{"tab", "switch between description/comments"},
		{"j/k or up/down", "scroll focused pane one line"},
		{"n/p", "jump to next/prev comment"},
		{"o", "open link(s) in browser"},
		{"f", "attachments (list/download/attach)"},
		{"c", "add comment"},
		{"e", "edit selected comment (own only)"},
		{"r", "refresh"},
		{"esc/backspace", "back"},
	}},
	{"Editor (comment / new item title)", [][2]string{
		{"enter", "save"},
		{"alt+enter", "insert newline"},
		{"esc", "cancel"},
	}},
	{"Editor (description)", [][2]string{
		{"ctrl+s", "save"},
		{"esc", "cancel"},
	}},
	{"Picker", [][2]string{
		{"j/k", "move"},
		{"enter", "apply"},
		{"esc", "cancel"},
	}},
	{"Searchable picker (state/labels/assignee)", [][2]string{
		{"(type)", "filter the list"},
		{"up/down", "move"},
		{"space", "toggle label (labels only)"},
		{"enter", "apply/save"},
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
