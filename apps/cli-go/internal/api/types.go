// Package api is a client for Plane's public REST API (/api/v1/).
package api

// User is the authenticated user, as returned by GET /api/v1/users/me/.
type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DisplayName string `json:"display_name"`
}

// Project is a Plane project (a "board" in the task's terminology).
type Project struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
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

// WorkItem is a Plane issue.
type WorkItem struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	DescriptionHTML string   `json:"description_html"`
	SequenceID      int      `json:"sequence_id"`
	State           string   `json:"state"`
	Priority        string   `json:"priority"`
	Assignees       []string `json:"assignees"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// Priorities are the valid values of WorkItem.Priority, in display order.
var Priorities = []string{"urgent", "high", "medium", "low", "none"}

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
