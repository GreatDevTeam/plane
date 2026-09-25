package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
	"github.com/makeplane/plane/apps/cli-go/internal/auth"
	"github.com/makeplane/plane/apps/cli-go/internal/logging"
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

// boardItemOrderBy sorts a board's work-item pages by state group (backlog/unstarted/
// started/completed/cancelled — see sortStates), the same left-to-right order the columns
// render in, so that as pages stream in, cards tend to fill their columns in board order
// rather than in whatever order the database returned them (its default is -created_at).
const boardItemOrderBy = "state__group"

// boardDataMsg carries the board's states plus the first page of work items. The board
// renders as soon as this arrives; remaining pages stream in afterwards via boardItemsPageMsg
// so a large project never blocks the whole board behind one request. Labels and members
// arrive separately (see fetchBoardExtras) rather than being bundled in here, so a slow
// labels/members request can never delay — or, worse, starve the timeout of — the items
// request that actually renders the board.
type boardDataMsg struct {
	states      []api.State
	items       []api.WorkItem
	nextCursor  string
	hasNextPage bool
	err         error
}

// fetchBoard fetches just enough to render the board: its states and the first page of work
// items. Each call gets its own fresh requestTimeout budget — previously both shared one
// context for the whole chain (states, items, labels, members in sequence), so a slow first
// call could starve the work-items request of any time at all, surfacing as a bare "context
// deadline exceeded" with no indication which request actually starved.
func fetchBoard(client *api.Client, workspaceSlug, projectID string) tea.Cmd {
	return func() tea.Msg {
		statesCtx, statesCancel := context.WithTimeout(context.Background(), requestTimeout)
		defer statesCancel()
		states, err := client.ListStates(statesCtx, workspaceSlug, projectID)
		if err != nil {
			return boardDataMsg{err: err}
		}

		itemsCtx, itemsCancel := context.WithTimeout(context.Background(), requestTimeout)
		defer itemsCancel()
		items, nextCursor, hasNext, err := client.ListWorkItemsPage(itemsCtx, workspaceSlug, projectID, "", boardItemOrderBy)
		if err != nil {
			return boardDataMsg{err: err}
		}
		return boardDataMsg{states: states, items: items, nextCursor: nextCursor, hasNextPage: hasNext}
	}
}

// boardExtrasMsg carries a board's labels and members — fetched apart from fetchBoard so they
// never sit on the critical path of showing the board itself (see boardDataMsg). projectID is
// which project this is for, so a slow answer that lands after the user has already switched
// projects again is not mistaken for the one now on screen (see applyBoardExtras).
type boardExtrasMsg struct {
	projectID string
	labels    []api.Label
	members   []api.Member
}

// fetchBoardExtras fetches a board's labels and members. Best-effort: a project without
// label/member read access still shows a working board, just without those two filters
// populated — a failure here never reaches setError/the footer, so it is logged as a warning
// instead of disappearing entirely.
func fetchBoardExtras(client *api.Client, workspaceSlug, projectID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		labels, err := client.ListLabels(ctx, workspaceSlug, projectID)
		if err != nil {
			logging.Warn("fetch labels for project %s: %v", projectID, err)
		}
		members, err := client.ListMembers(ctx, workspaceSlug, projectID)
		if err != nil {
			logging.Warn("fetch members for project %s: %v", projectID, err)
		}
		return boardExtrasMsg{projectID: projectID, labels: labels, members: members}
	}
}

// extrasCacheEntry is one project's last-known labels/members plus when they were fetched —
// see Model.extrasCache.
type extrasCacheEntry struct {
	labels    []api.Label
	members   []api.Member
	fetchedAt time.Time
}

// extrasCacheTTL is how stale a project's cached labels/members are allowed to get before a
// board reopen or the 30s auto-refresh fetches a fresh copy — the assignee/label pickers still
// show the cached copy immediately either way (see applyBoardExtras), this only bounds how
// often the request itself goes out.
const extrasCacheTTL = 5 * time.Minute

type workItemUpdatedMsg struct {
	item *api.WorkItem
	err  error
}

// updateWorkItem PATCHes the given fields and then, once that succeeds, re-fetches the work
// item with its own GET rather than trusting the PATCH response as the new state to show: the
// board and detail screen must only ever change what they display after a request that actually
// reflects it has come back, and a follow-up GET is the authoritative read for that, the same
// one openWorkItem/handleWorkItemLoaded already trust for a card just opened from the board. On
// a PATCH failure the GET is skipped — there's nothing new to read yet.
func updateWorkItem(client *api.Client, workspaceSlug, projectID, workItemID string, patch map[string]any) tea.Cmd {
	return func() tea.Msg {
		patchCtx, patchCancel := context.WithTimeout(context.Background(), requestTimeout)
		_, err := client.UpdateWorkItem(patchCtx, workspaceSlug, projectID, workItemID, patch)
		patchCancel()
		if err != nil {
			return workItemUpdatedMsg{err: err}
		}
		getCtx, getCancel := context.WithTimeout(context.Background(), requestTimeout)
		defer getCancel()
		item, err := client.GetWorkItem(getCtx, workspaceSlug, projectID, workItemID)
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
		// Every page of one board load must ask for the same order_by as the first page:
		// Plane's cursor encodes a position in that ordering, so switching it mid-pagination
		// would produce duplicate or skipped items.
		items, nextCursor, hasNext, err := client.ListWorkItemsPage(ctx, workspaceSlug, projectID, cursor, boardItemOrderBy)
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
