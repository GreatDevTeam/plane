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
