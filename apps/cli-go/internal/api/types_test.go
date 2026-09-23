package api

import (
	"encoding/json"
	"testing"
)

func TestMemberName(t *testing.T) {
	cases := []struct {
		name string
		m    Member
		want string
	}{
		{"display name wins", Member{DisplayName: "Alice A", FirstName: "Alice", Email: "a@x.com"}, "Alice A"},
		{"falls back to first/last", Member{FirstName: "Bob", LastName: "Jones", Email: "b@x.com"}, "Bob Jones"},
		{"falls back to email", Member{Email: "c@x.com"}, "c@x.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.Name(); got != tc.want {
				t.Errorf("Name() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMemberFullName(t *testing.T) {
	cases := []struct {
		name string
		m    Member
		want string
	}{
		{"real name wins over the handle", Member{DisplayName: "jane.doe", FirstName: "Jane", LastName: "Doe"}, "Jane Doe"},
		{"first name only", Member{DisplayName: "jane.doe", FirstName: "Jane"}, "Jane"},
		{"falls back to the handle", Member{DisplayName: "jane.doe", Email: "jane@x.com"}, "jane.doe"},
		{"falls back to the email", Member{Email: "jane@x.com"}, "jane@x.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.FullName(); got != tc.want {
				t.Errorf("FullName() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSortStatesMatchesWebOrder pins the board column order to the web app's: states are
// ordered by their group's position in StateGroups first and only then by sequence (see
// sortStates in packages/utils/src/work-item/state.ts). Sorting by sequence alone — which is
// what the CLI used to do — interleaves the groups, so the same board came out in a different
// order in the terminal than in the browser.
func TestSortStatesMatchesWebOrder(t *testing.T) {
	states := []State{
		{Name: "Done", Group: "completed", Sequence: 1000},
		{Name: "Cancelled", Group: "cancelled", Sequence: 500},
		{Name: "In Review", Group: "started", Sequence: 30000},
		{Name: "In Progress", Group: "started", Sequence: 20000},
		{Name: "Todo", Group: "unstarted", Sequence: 65535},
		{Name: "Backlog", Group: "backlog", Sequence: 99999},
		{Name: "Triage", Group: "triage", Sequence: 1},
	}
	sortStates(states)

	got := make([]string, len(states))
	for i, st := range states {
		got[i] = st.Name
	}
	want := []string{"Backlog", "Todo", "In Progress", "In Review", "Done", "Cancelled", "Triage"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sortStates order = %v, want %v", got, want)
		}
	}
}

func TestPriorityRank(t *testing.T) {
	if got, want := PriorityRank("urgent"), 0; got != want {
		t.Errorf("PriorityRank(urgent) = %d, want %d", got, want)
	}
	if PriorityRank("") != PriorityRank("none") {
		t.Errorf("an empty priority must rank as %q", "none")
	}
	if PriorityRank("low") >= PriorityRank("none") {
		t.Errorf("low must sort before none")
	}
	if PriorityRank("nonsense") < PriorityRank("none") {
		t.Errorf("an unknown priority must sort last")
	}
}

// TestWorkItemParentUnmarshal pins the shape Plane actually sends for a work item's parent:
// the REST API serializes it as a bare work item UUID, and as JSON null for a top-level
// item — which unmarshals into the zero value, so "" is what "no parent" looks like in Go.
// There is no matching children field in the payload at all (sub_issues_count is annotated
// on the API's queryset but never serialized), which is why the client derives sub-tasks by
// scanning the project's own work items.
func TestWorkItemParentUnmarshal(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"sub-task", `{"id":"a","parent":"11111111-2222-3333-4444-555555555555"}`, "11111111-2222-3333-4444-555555555555"},
		{"top-level item", `{"id":"a","parent":null}`, ""},
		{"field absent", `{"id":"a"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var wi WorkItem
			if err := json.Unmarshal([]byte(tc.body), &wi); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if wi.Parent != tc.want {
				t.Errorf("Parent = %q, want %q", wi.Parent, tc.want)
			}
		})
	}
}
