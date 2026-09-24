// Package api is a client for Plane's public REST API (/api/v1/).
package api

import (
	"sort"
	"strings"
)

// User is the authenticated user, as returned by GET /api/v1/users/me/.
type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DisplayName string `json:"display_name"`
}

// Workspace is a Plane workspace the signed-in user belongs to.
type Workspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Project is a Plane project (a "board" in the task's terminology).
type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

// Label is a work item label.
type Label struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Member is a project member (used to resolve assignee IDs to display names).
type Member struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DisplayName string `json:"display_name"`
}

// Name returns the best available display name for a member. Plane's display_name is
// usually a handle ("jane.doe"), which is what compact lists (filters, pickers) want.
func (m Member) Name() string {
	if m.DisplayName != "" {
		return m.DisplayName
	}
	if m.FirstName != "" || m.LastName != "" {
		return strings.TrimSpace(m.FirstName + " " + m.LastName)
	}
	return m.Email
}

// FullName returns the member's real name ("Jane Doe") where the account has one, falling
// back to the display-name handle and then the email. Used wherever there is room for a
// person's actual name rather than a handle, e.g. the work item detail screen.
func (m Member) FullName() string {
	if full := strings.TrimSpace(m.FirstName + " " + m.LastName); full != "" {
		return full
	}
	if m.DisplayName != "" {
		return m.DisplayName
	}
	return m.Email
}

// State is a work item state (a kanban column), e.g. Backlog / Todo / In Progress / Done / Cancelled.
type State struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Color    string  `json:"color"`
	Group    string  `json:"group"`
	Sequence float64 `json:"sequence"`
	Default  bool    `json:"default"`
}

// StateGroups are Plane's fixed state groups, in the order the web app lays its board
// columns out (see sortStates in packages/utils/src/work-item/state.ts, which orders states
// by their group's index in STATE_GROUPS before their sequence).
var StateGroups = []string{"backlog", "unstarted", "started", "completed", "cancelled"}

// stateGroupIndex is a state group's position in StateGroups. An unknown group (e.g.
// "triage", which the web board does not show as a column) sorts after every known one
// rather than jumping to the front.
func stateGroupIndex(group string) int {
	for i, g := range StateGroups {
		if g == group {
			return i
		}
	}
	return len(StateGroups)
}

// sortStates orders states the way the web app's board does: by their group's position in
// StateGroups first, then by sequence inside the group. Sorting by sequence alone (which is
// what the CLI used to do) interleaves the groups, so a board's columns came out in a
// different order than the same board in the browser.
func sortStates(states []State) {
	sort.SliceStable(states, func(i, j int) bool {
		gi, gj := stateGroupIndex(states[i].Group), stateGroupIndex(states[j].Group)
		if gi != gj {
			return gi < gj
		}
		return states[i].Sequence < states[j].Sequence
	})
}

// WorkItem is a Plane issue.
type WorkItem struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	DescriptionHTML string `json:"description_html"`
	SequenceID      int    `json:"sequence_id"`
	State           string `json:"state"`
	// Parent is the work item this one is a sub-task of, or "" when it has none — Plane
	// sends `null` there, which json.Unmarshal leaves as the zero value. There is no
	// matching "children" field: the API never serializes one (its sub_issues_count
	// annotation stays server-side) and its list endpoint has no parent=<id> filter, so
	// callers derive sub-tasks by scanning the project's own work items for this ID.
	Parent    string   `json:"parent"`
	Priority  string   `json:"priority"`
	Assignees []string `json:"assignees"`
	Labels    []string `json:"labels"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// Priorities are the valid values of WorkItem.Priority, in display order.
var Priorities = []string{"urgent", "high", "medium", "low", "none"}

// PriorityRank is a priority's position in Priorities (urgent first). An empty priority is
// Plane's "none", and anything unrecognised sorts last.
func PriorityRank(priority string) int {
	if priority == "" {
		priority = "none"
	}
	for i, p := range Priorities {
		if p == priority {
			return i
		}
	}
	return len(Priorities)
}

// Comment is a comment on a work item.
type Comment struct {
	ID          string `json:"id"`
	CommentHTML string `json:"comment_html"`
	Actor       string `json:"actor"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	EditedAt    string `json:"edited_at"`
}

// Attachment is a file attached to a work item (FileAsset, as returned by the
// work-items/{id}/attachments/ endpoints). Its display fields live under "attributes" rather
// than at the top level.
type Attachment struct {
	ID         string `json:"id"`
	CreatedAt  string `json:"created_at"`
	Attributes struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Size int64  `json:"size"`
	} `json:"attributes"`
}

// Name is the attachment's original filename.
func (a Attachment) Name() string { return a.Attributes.Name }

// Size is the attachment's size in bytes.
func (a Attachment) Size() int64 { return a.Attributes.Size }

// paginatedResponse is the envelope every Plane list endpoint returns.
type paginatedResponse[T any] struct {
	Results         []T    `json:"results"`
	TotalCount      int    `json:"total_count"`
	NextCursor      string `json:"next_cursor"`
	NextPageResults bool   `json:"next_page_results"`
}

// apiError is the body Plane returns on a non-2xx response.
type apiError struct {
	Error  string `json:"error"`
	Detail string `json:"detail"`
}
