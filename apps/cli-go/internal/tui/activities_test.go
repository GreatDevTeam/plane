package tui

import (
	"testing"

	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// TestActivitySummary covers the field/verb combinations activitySummary turns into a
// one-line, human-readable description — old_value/new_value already arrive as display names
// (see apps/api/plane/bgtasks/issue_activities_task.py), never IDs, so this never resolves
// anything itself.
func TestActivitySummary(t *testing.T) {
	cases := []struct {
		name string
		a    api.Activity
		want string
	}{
		{"created", api.Activity{Comment: "created the issue"}, "created the issue"},
		{"created without comment", api.Activity{}, "created the work item"},
		{"renamed", api.Activity{Field: "name", NewValue: "New title"}, `renamed to "New title"`},
		{"description", api.Activity{Field: "description", NewValue: "<p>x</p>"}, "updated the description"},
		{"state changed", api.Activity{Field: "state", OldValue: "Todo", NewValue: "Done"}, "changed state from Todo to Done"},
		{"state set", api.Activity{Field: "state", NewValue: "Todo"}, "changed state to Todo"},
		{"priority set", api.Activity{Field: "priority", NewValue: "high"}, "changed priority to high"},
		{"priority cleared", api.Activity{Field: "priority"}, "cleared the priority"},
		{"label added", api.Activity{Field: "labels", NewValue: "bug"}, "added label bug"},
		{"label removed", api.Activity{Field: "labels", OldValue: "bug"}, "removed label bug"},
		{"assignee added", api.Activity{Field: "assignees", NewValue: "Jane"}, "assigned Jane"},
		{"assignee removed", api.Activity{Field: "assignees", OldValue: "Jane"}, "unassigned Jane"},
		{"unknown field", api.Activity{Field: "cycles", NewValue: "Sprint 1"}, "changed cycles to Sprint 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := activitySummary(c.a); got != c.want {
				t.Errorf("activitySummary(%+v) = %q, want %q", c.a, got, c.want)
			}
		})
	}
}
