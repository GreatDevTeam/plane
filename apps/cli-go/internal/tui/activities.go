package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

type activitiesMsg struct {
	items []api.Activity
	err   error
}

func fetchActivities(client *api.Client, workspaceSlug, projectID, workItemID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		items, err := client.ListActivities(ctx, workspaceSlug, projectID, workItemID)
		return activitiesMsg{items: items, err: err}
	}
}

// handleActivities lands the activity/history log fetched alongside comments (see
// openWorkItem, refreshDetail, handleBoardTick). A failure here (e.g. an older server without
// this endpoint) is silent: the log is supplementary to the comment thread the pane is named
// after, so it must not blank comments already on screen or surface its own error banner.
func (m Model) handleActivities(msg activitiesMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, nil
	}
	m.activities = msg.items
	return m, nil
}

// activitySummary renders one activity log entry as the single line the comments pane shows
// it as. Every field's old/new values already arrive as the human-readable name the web app's
// own activity feed shows (state/label/priority names, not IDs — see
// apps/api/plane/bgtasks/issue_activities_task.py), so no ID resolution happens here.
func activitySummary(a api.Activity) string {
	switch a.Field {
	case "", "issue":
		if a.Comment != "" {
			return a.Comment
		}
		return "created the work item"
	case "name":
		return fmt.Sprintf("renamed to %q", a.NewValue)
	case "description":
		return "updated the description"
	case "state":
		if a.OldValue != "" && a.NewValue != "" {
			return fmt.Sprintf("changed state from %s to %s", a.OldValue, a.NewValue)
		}
		return fmt.Sprintf("changed state to %s", a.NewValue)
	case "priority":
		if a.NewValue == "" {
			return "cleared the priority"
		}
		return fmt.Sprintf("changed priority to %s", a.NewValue)
	case "labels":
		if a.NewValue != "" {
			return fmt.Sprintf("added label %s", a.NewValue)
		}
		return fmt.Sprintf("removed label %s", a.OldValue)
	case "assignees":
		if a.NewValue != "" {
			return fmt.Sprintf("assigned %s", a.NewValue)
		}
		return fmt.Sprintf("unassigned %s", a.OldValue)
	case "parent":
		if a.NewValue != "" {
			return fmt.Sprintf("set parent to %s", a.NewValue)
		}
		return "removed the parent"
	case "target_date", "start_date":
		label := strings.ReplaceAll(a.Field, "_", " ")
		if a.NewValue == "" {
			return fmt.Sprintf("cleared the %s", label)
		}
		return fmt.Sprintf("changed %s to %s", label, a.NewValue)
	default:
		if a.NewValue != "" {
			return fmt.Sprintf("changed %s to %s", a.Field, a.NewValue)
		}
		return fmt.Sprintf("changed %s", a.Field)
	}
}
