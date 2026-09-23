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

type boardDataMsg struct {
	states  []api.State
	items   []api.WorkItem
	labels  []api.Label
	members []api.Member
	err     error
}

func fetchBoard(client *api.Client, workspaceSlug, projectID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		states, err := client.ListStates(ctx, workspaceSlug, projectID)
		if err != nil {
			return boardDataMsg{err: err}
		}
		items, err := client.ListWorkItems(ctx, workspaceSlug, projectID)
		if err != nil {
			return boardDataMsg{err: err}
		}
		// Best-effort: a project without label/member read access still shows a working
		// board, just without those two filters populated.
		labels, _ := client.ListLabels(ctx, workspaceSlug, projectID)
		members, _ := client.ListMembers(ctx, workspaceSlug, projectID)
		return boardDataMsg{states: states, items: items, labels: labels, members: members}
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
