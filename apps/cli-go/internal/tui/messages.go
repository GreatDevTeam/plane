package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
	"github.com/makeplane/plane/apps/cli-go/internal/auth"
)

const requestTimeout = 20 * time.Second

type loginResultMsg struct {
	client     *api.Client
	user       *api.User
	token      string
	workspaces []api.Workspace
	err        error
}

func loginWithToken(serverURL, token string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		client, user, err := auth.TokenLogin(ctx, serverURL, token)
		return loginResultMsg{client: client, user: user, token: token, err: err}
	}
}

func loginWithPassword(serverURL, email, password string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		client, user, token, workspaces, err := auth.PasswordLogin(ctx, serverURL, email, password)
		return loginResultMsg{client: client, user: user, token: token, workspaces: workspaces, err: err}
	}
}

type projectsMsg struct {
	projects []api.Project
	err      error
}

func fetchProjects(client *api.Client, workspaceSlug string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		projects, err := client.ListProjects(ctx, workspaceSlug)
		return projectsMsg{projects: projects, err: err}
	}
}

// boardDataMsg carries the board's states/labels/members plus the first page of work
// items. The board renders as soon as this arrives; remaining pages stream in afterwards
// via boardItemsPageMsg so a large project never blocks the whole board behind one request.
type boardDataMsg struct {
	states      []api.State
	items       []api.WorkItem
	nextCursor  string
	hasNextPage bool
	labels      []api.Label
	members     []api.Member
	err         error
}

func fetchBoard(client *api.Client, workspaceSlug, projectID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		states, err := client.ListStates(ctx, workspaceSlug, projectID)
		if err != nil {
			return boardDataMsg{err: err}
		}
		items, nextCursor, hasNext, err := client.ListWorkItemsPage(ctx, workspaceSlug, projectID, "")
		if err != nil {
			return boardDataMsg{err: err}
		}
		// Best-effort: a project without label/member read access still shows a working
		// board, just without those two filters populated.
		labels, _ := client.ListLabels(ctx, workspaceSlug, projectID)
		members, _ := client.ListMembers(ctx, workspaceSlug, projectID)
		return boardDataMsg{states: states, items: items, nextCursor: nextCursor, hasNextPage: hasNext, labels: labels, members: members}
	}
}

type workItemUpdatedMsg struct {
	item *api.WorkItem
	err  error
}

func updateWorkItem(client *api.Client, workspaceSlug, projectID, workItemID string, patch map[string]any) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		item, err := client.UpdateWorkItem(ctx, workspaceSlug, projectID, workItemID, patch)
		return workItemUpdatedMsg{item: item, err: err}
	}
}

// workItemCreatedMsg carries the result of creating a new work item from the board.
type workItemCreatedMsg struct {
	item *api.WorkItem
	err  error
}

func createWorkItem(client *api.Client, workspaceSlug, projectID string, fields map[string]any) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		item, err := client.CreateWorkItem(ctx, workspaceSlug, projectID, fields)
		return workItemCreatedMsg{item: item, err: err}
	}
}

// boardItemsPageMsg carries a single page of work items so the board can render as soon as
// the first page arrives, instead of blocking on the whole project.
type boardItemsPageMsg struct {
	items      []api.WorkItem
	nextCursor string
	hasNext    bool
	err        error
}

func fetchWorkItemsPage(client *api.Client, workspaceSlug, projectID, cursor string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		items, nextCursor, hasNext, err := client.ListWorkItemsPage(ctx, workspaceSlug, projectID, cursor)
		return boardItemsPageMsg{items: items, nextCursor: nextCursor, hasNext: hasNext, err: err}
	}
}

type commentsMsg struct {
	items []api.Comment
	err   error
}

func fetchComments(client *api.Client, workspaceSlug, projectID, workItemID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		items, err := client.ListComments(ctx, workspaceSlug, projectID, workItemID)
		return commentsMsg{items: items, err: err}
	}
}

type commentSavedMsg struct {
	comment *api.Comment
	editing bool
	err     error
}

func createComment(client *api.Client, workspaceSlug, projectID, workItemID, commentHTML string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		cm, err := client.CreateComment(ctx, workspaceSlug, projectID, workItemID, commentHTML)
		return commentSavedMsg{comment: cm, err: err}
	}
}

func editComment(client *api.Client, workspaceSlug, projectID, workItemID, commentID, commentHTML string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		cm, err := client.UpdateComment(ctx, workspaceSlug, projectID, workItemID, commentID, commentHTML)
		return commentSavedMsg{comment: cm, editing: true, err: err}
	}
}

// boardRefreshInterval is how often the board re-fetches itself in the background.
const boardRefreshInterval = 30 * time.Second

// refreshTimeout is the deadline for a background refresh. It is more generous than
// requestTimeout because one refresh walks every page of a project's work items, so that the
// whole board can be swapped in at once instead of page by page (which is what would make it
// blink).
const refreshTimeout = 60 * time.Second

// boardTickMsg fires every boardRefreshInterval while a board is open.
type boardTickMsg time.Time

func boardTick() tea.Cmd {
	return tea.Tick(boardRefreshInterval, func(t time.Time) tea.Msg { return boardTickMsg(t) })
}

// boardRefreshedMsg carries a complete, freshly fetched board. Unlike boardDataMsg it is
// applied on top of the board already on screen: the states and items are swapped in one go,
// keeping the focused column, the cursors and the filters, so a refresh is invisible unless
// something actually changed.
type boardRefreshedMsg struct {
	states []api.State
	items  []api.WorkItem
	labels []api.Label
	err    error
}

func refreshBoard(client *api.Client, workspaceSlug, projectID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
		defer cancel()
		states, err := client.ListStates(ctx, workspaceSlug, projectID)
		if err != nil {
			return boardRefreshedMsg{err: err}
		}
		items, err := client.ListWorkItems(ctx, workspaceSlug, projectID)
		if err != nil {
			return boardRefreshedMsg{err: err}
		}
		labels, _ := client.ListLabels(ctx, workspaceSlug, projectID)
		return boardRefreshedMsg{states: states, items: items, labels: labels}
	}
}

// relatedWorkItemMsg carries a work item fetched purely because something on screen refers
// to it — a parent the board's own item list does not have (see fetchMissingParent). It is
// kept apart from workItemLoadedMsg so that landing one never swaps the work item the user
// is actually looking at.
type relatedWorkItemMsg struct {
	item *api.WorkItem
	err  error
}

func fetchRelatedWorkItem(client *api.Client, workspaceSlug, projectID, workItemID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		item, err := client.GetWorkItem(ctx, workspaceSlug, projectID, workItemID)
		return relatedWorkItemMsg{item: item, err: err}
	}
}

// workItemLoadedMsg carries the freshly fetched copy of the work item the detail screen has
// open — the background half of opening a card from the board's cache.
type workItemLoadedMsg struct {
	item *api.WorkItem
	err  error
}

func fetchWorkItem(client *api.Client, workspaceSlug, projectID, workItemID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		item, err := client.GetWorkItem(ctx, workspaceSlug, projectID, workItemID)
		return workItemLoadedMsg{item: item, err: err}
	}
}
