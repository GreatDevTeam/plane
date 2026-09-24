package tui

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// Parent / sub-task relations.
//
// Plane's public REST API sends a work item's `parent` (a work item ID, or null) but never
// its children: the sub_issues_count annotation in the API's queryset is not a serializer
// field, and the list endpoint has no parent=<id> filter to ask for them with. Children are
// therefore derived from the board's own item list, which is a complete source for them —
// the board pages through every work item in the project, and the API refuses a parent from
// a different project (IssueSerializer.validate), so a sub-task is always in the same
// project as its parent.
//
// The one thing that list can be missing is the *parent* of an item the user opened: it may
// have been archived (Issue.issue_objects hides those) or its page may not have landed yet.
// That single item is fetched by ID on demand and cached in Model.relatedItems.

// lookupItem finds a work item by ID in the board's list, falling back to the on-demand
// cache of items fetched because they were not in it. Returns nil when neither has it.
func (m Model) lookupItem(id string) *api.WorkItem {
	if id == "" {
		return nil
	}
	for i := range m.items {
		if m.items[i].ID == id {
			return &m.items[i]
		}
	}
	if it, ok := m.relatedItems[id]; ok {
		return &it
	}
	return nil
}

// parentOf is the work item `it` is a sub-task of, or nil when it has no parent (or the
// parent is not known to this client yet — see fetchMissingParent).
func (m Model) parentOf(it api.WorkItem) *api.WorkItem {
	return m.lookupItem(it.Parent)
}

// subIssues are the work items that name id as their parent, ordered by work item number so
// the list reads the same way twice running (m.items arrives in whatever order the API
// paged it in, and a background refresh can reorder it).
func (m Model) subIssues(id string) []api.WorkItem {
	if id == "" {
		return nil
	}
	var out []api.WorkItem
	for _, it := range m.items {
		if it.Parent == id {
			out = append(out, it)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SequenceID < out[j].SequenceID })
	return out
}

// subIssueCount is how many work items name id as their parent. viewBoard precomputes every
// card's count once per frame (see childCounts) because otherwise each of the ~100 cards on
// screen would rescan the whole project's item list; the scan is kept as a fallback so a
// Model built without that map still renders the right badge.
func (m Model) subIssueCount(id string) int {
	if id == "" {
		return 0
	}
	if m.subCounts != nil {
		return m.subCounts[id]
	}
	n := 0
	for i := range m.items {
		if m.items[i].Parent == id {
			n++
		}
	}
	return n
}

// childCounts is the number of sub-tasks of every work item that has any, in one pass over
// the board's item list.
func (m Model) childCounts() map[string]int {
	counts := make(map[string]int)
	for i := range m.items {
		if p := m.items[i].Parent; p != "" {
			counts[p]++
		}
	}
	return counts
}

// stateName resolves a state ID to its name, the way both the board and the detail screen
// want to show it.
func (m Model) stateName(id string) string {
	if st := m.findState(id); st != nil {
		return st.Name
	}
	return "unknown"
}

// relationSummary is how a parent or a sub-task is listed on the detail screen: its work
// item number and title, then its state and priority — showing those is the whole point of
// listing a related item rather than just linking its title.
func (m Model) relationSummary(it api.WorkItem) string {
	return fmt.Sprintf("#%d %s  [%s · %s]", it.SequenceID, oneLine(it.Name), m.stateName(it.State), m.priorityLabel(it.Priority))
}

// cardRelations is the compact parent/sub-task badge a board card carries on its meta line:
// a bare "↑" for the parent it belongs to and "↳3" for the sub-tasks hanging off it. It stays
// on the existing meta line on purpose — a fourth card row would cost every column a quarter
// of its cards (see cardRows).
//
// The parent badge carries no work item number — a board column is already tight on width
// (see cardMetaLine's own comment on what gets cut first), and the number is one "g" away on
// the detail screen's own Parent: line (parentLine), which is where it is actually useful for
// jumping to it. "↑" alone is enough to say the card has one.
func (m Model) cardRelations(it api.WorkItem) string {
	out := ""
	if it.Parent != "" {
		out = "↑"
	}
	if n := m.subIssueCount(it.ID); n > 0 {
		if out != "" {
			out += " "
		}
		out += fmt.Sprintf("↳%d", n)
	}
	return out
}

// openWorkItem switches the detail screen onto it, the same way pressing enter on a board
// card does: the copy already in hand is shown straight away and the server's copy (plus the
// comments) is fetched behind it. Shared by the board and by the parent/sub-task jumps, so
// arriving at a work item always leaves the screen in the same state.
func (m Model) openWorkItem(it api.WorkItem) (tea.Model, tea.Cmd) {
	m.focusBoardOn(it)
	m.detailItem = &it
	m.comments = nil
	m.commentCursor = 0
	m.commentsLoading = true
	m.detailLoading = true
	m.attachments = nil
	m.attachmentsLoading = true
	m.pickerOpen = ""
	m.resetDetailView()
	m.screen = screenDetail
	cmds := []tea.Cmd{
		fetchWorkItem(m.client, m.workspaceSlug, m.project.ID, it.ID),
		fetchComments(m.client, m.workspaceSlug, m.project.ID, it.ID),
		fetchAttachments(m.client, m.workspaceSlug, m.project.ID, it.ID),
	}
	if cmd := m.fetchMissingParent(it); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// focusBoardOn moves the board's focused column and cursor onto it, so backing out of the
// detail screen lands on the card the user was just looking at rather than wherever they
// started. An item the board is not showing (filtered out, or not loaded) leaves the board
// alone.
func (m *Model) focusBoardOn(it api.WorkItem) {
	for ci, st := range m.states {
		if st.ID != it.State || ci >= len(m.colCursor) {
			continue
		}
		for ii, col := range m.columnItems(st.ID) {
			if col.ID == it.ID {
				m.focusedCol = ci
				m.colCursor[ci] = ii
				return
			}
		}
		return
	}
}

// fetchMissingParent asks the server for the parent of it when the board's item list does not
// have it — an archived parent, or one on a page that has not arrived yet. Returns nil when
// there is nothing to fetch.
func (m Model) fetchMissingParent(it api.WorkItem) tea.Cmd {
	if it.Parent == "" || m.client == nil || m.lookupItem(it.Parent) != nil {
		return nil
	}
	return fetchRelatedWorkItem(m.client, m.workspaceSlug, m.project.ID, it.Parent)
}

// handleRelatedWorkItem caches a work item fetched only because it was referenced as a
// parent. A failure is deliberately silent: the detail screen already renders without it
// (the parent line says the item is unavailable), and an error banner for a background
// enrichment would cover up whatever the user was actually doing.
func (m Model) handleRelatedWorkItem(msg relatedWorkItemMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil || msg.item == nil {
		return m, nil
	}
	related := make(map[string]api.WorkItem, len(m.relatedItems)+1)
	for k, v := range m.relatedItems {
		related[k] = v
	}
	related[msg.item.ID] = *msg.item
	m.relatedItems = related
	return m, nil
}

// jumpToParent opens the parent of the work item on screen. It is a no-op for a top-level
// item, and reports the one case the user cannot act on: a parent this client cannot see.
func (m Model) jumpToParent() (tea.Model, tea.Cmd) {
	if m.detailItem == nil || m.detailItem.Parent == "" {
		return m, nil
	}
	parent := m.parentOf(*m.detailItem)
	if parent == nil {
		m.setError(fmt.Errorf("parent work item is not available (it may be archived)"))
		return m, nil
	}
	m.setError(nil)
	return m.openWorkItem(*parent)
}

// openSubIssuePicker lists the sub-tasks of the work item on screen for the user to jump to.
func (m *Model) openSubIssuePicker() {
	if m.detailItem == nil {
		return
	}
	if len(m.subIssues(m.detailItem.ID)) == 0 {
		m.setError(fmt.Errorf("this work item has no sub-tasks"))
		return
	}
	m.setError(nil)
	m.pickerOpen = "subissue"
	m.pickerIdx = 0
}

// subIssueOptions are the sub-tasks the "subissue" picker lists, in the same order the
// detail screen's meta block lists them.
func (m Model) subIssueOptions() []api.WorkItem {
	if m.detailItem == nil {
		return nil
	}
	return m.subIssues(m.detailItem.ID)
}

// maxSubIssuePickerRows / minSubIssuePickerRows bound how many sub-tasks the jump picker
// lists at once. Unlike the state/priority lists it is fed by project data with no upper
// bound, and the picker is drawn below the detail screen's two panes (detailBottom) — an
// unwindowed list of 30 children would push its own bottom, and the "esc cancel" hint with
// it, off the terminal.
const (
	maxSubIssuePickerRows = 8
	minSubIssuePickerRows = 3
)

// subIssuePickerRows is how many sub-tasks the jump picker lists on this terminal: whatever
// is left once everything drawn around it has been paid for — the title and meta block, each
// pane's header and the blank lines between them, the two panes at their floor, the footer,
// and the picker's own border, title and hint rows. Budgeting it this way is what keeps the
// picker whole on a short terminal, where clipRows would otherwise cut its last options off.
func (m Model) subIssuePickerRows() int {
	width, height := m.termSize()
	// detailMeta drops its inline sub-task list while a picker is open (see subTaskLines),
	// so this measures the meta block at the size it will actually be drawn at.
	fixed := visualHeight(m.detailHeader(), width) + 1 +
		visualHeight(m.detailMeta(width), width) + 1 +
		1 /* Description header */ + 1 /* blank line between panes */ +
		1 /* Comments header */ + 1 /* blank line before the bottom */ +
		m.minPaneRows()*2 +
		visualHeight(m.footer(""), width) /* the hints go while a picker is open */ +
		2 /* the blank line between the footer and the picker */ +
		4 /* the picker's own border, title and hint rows */
	rows := height - fixed
	if rows > maxSubIssuePickerRows {
		rows = maxSubIssuePickerRows
	}
	if rows < minSubIssuePickerRows {
		rows = minSubIssuePickerRows
	}
	return rows
}

// pickerWindow is the slice of a list of n options that a windowed picker shows with option
// `idx` selected: at most max options, centred on the selection, plus whether anything was
// left out (so the picker can say the list is scrolled). The same arithmetic renderColumn
// uses to scroll a column around its cursor.
func pickerWindow(n, idx, max int) (start, count int, scrolled bool) {
	if n <= max {
		return 0, n, false
	}
	start = idx - max/2
	if start+max > n {
		start = n - max
	}
	if start < 0 {
		start = 0
	}
	return start, max, true
}
