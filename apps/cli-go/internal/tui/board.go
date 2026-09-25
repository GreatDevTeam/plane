package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
	"github.com/makeplane/plane/apps/cli-go/internal/config"
)

func (m Model) handleBoardData(msg boardDataMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.status = ""
	if msg.err != nil {
		m.setError(msg.err)
		m.screen = screenProjects
		return m, nil
	}
	m.setError(nil)
	m.states = msg.states
	m.items = msg.items
	m.itemsNextCursor = msg.nextCursor
	// A new board's labels/members are fetched separately (fetchBoardExtras, batched
	// alongside fetchBoard by updateProjects) so they never delay this render. A project
	// visited earlier this session shows its cached copy straight away instead of going blank
	// until a fresh one lands — see extrasCache and the assignee-picker bug that motivated it.
	if entry, ok := m.extrasCache[m.project.ID]; ok {
		m.labels = entry.labels
		m.members = entry.members
	} else {
		m.labels = nil
		m.members = nil
	}
	m.focusedCol = 0
	m.colCursor = make([]int, len(m.states))
	m.filterAssignee = ""
	m.filterLabel = ""
	m.filterState = ""
	m.filterPriority = ""
	m.titleSearch = ""
	m.hiddenStates = m.cfg.HiddenStatesFor(m.project.ID)
	m.screen = screenBoard

	var cmds []tea.Cmd
	if msg.hasNextPage {
		m.itemsLoadingMore = true
		cmds = append(cmds, fetchWorkItemsPage(m.client, m.workspaceSlug, m.project.ID, msg.nextCursor))
	}
	// Start the 30s auto-refresh chain the first time a board is opened. Exactly one chain
	// runs for the lifetime of the process: re-arming it on every board load would stack a
	// second (then a third) ticker each time the user switches projects.
	if !m.autoRefreshOn {
		m.autoRefreshOn = true
		cmds = append(cmds, boardTick())
	}
	return m, tea.Batch(cmds...)
}

// handleBoardExtras applies a board's labels and members once fetchBoardExtras returns —
// independently of boardDataMsg, so a slow labels/members request never blocks the board
// itself from rendering (see fetchBoard).
func (m Model) handleBoardExtras(msg boardExtrasMsg) (tea.Model, tea.Cmd) {
	m.applyBoardExtras(msg)
	return m, nil
}

// applyBoardExtras records a fetched labels/members answer in the per-project cache — so
// switching back to this project later shows them immediately (see extrasCache) — and, only if
// the user is still looking at that same project, applies it to the board on screen. Guarding
// on msg.projectID matters because fetchBoardExtras can be in flight when the user switches
// projects again: without it, a slow answer for the project they left would land on top of the
// one they are now looking at.
func (m *Model) applyBoardExtras(msg boardExtrasMsg) {
	cache := make(map[string]extrasCacheEntry, len(m.extrasCache)+1)
	for k, v := range m.extrasCache {
		cache[k] = v
	}
	cache[msg.projectID] = extrasCacheEntry{labels: msg.labels, members: msg.members, fetchedAt: time.Now()}
	m.extrasCache = cache
	if msg.projectID == m.project.ID {
		m.labels = msg.labels
		m.members = msg.members
	}
}

// extrasStale reports whether a project's cached labels/members are missing or older than
// extrasCacheTTL, the cadence fetchBoardExtras is refreshed at outside of an explicit board
// open (see handleBoardTick).
func (m Model) extrasStale(projectID string) bool {
	entry, ok := m.extrasCache[projectID]
	if !ok {
		return true
	}
	return time.Since(entry.fetchedAt) >= extrasCacheTTL
}

// handleBoardTick fires every boardRefreshInterval. It re-arms itself unconditionally, so
// the board keeps refreshing after a skipped tick, and starts a background refresh unless
// something on screen would be disturbed by one (see shouldSkipRefresh). While the detail
// screen is open this also re-fetches its comments — applyBoardRefresh already keeps the open
// item's own fields in sync, but comments are a separate endpoint the board refresh never
// touches.
func (m Model) handleBoardTick(boardTickMsg) (tea.Model, tea.Cmd) {
	if m.shouldSkipRefresh() {
		return m, boardTick()
	}
	m.refreshing = true
	cmds := []tea.Cmd{refreshBoard(m.client, m.workspaceSlug, m.project.ID), boardTick()}
	// Members (unlike labels, which refreshBoard already re-fetches every tick) have no other
	// refresh path, so a permission error or a dropped response on the board's initial load
	// used to leave the assignee picker empty for the rest of the session. Retrying here bounds
	// that to extrasCacheTTL instead of never, without hitting the endpoint every 30s.
	if m.extrasStale(m.project.ID) {
		cmds = append(cmds, fetchBoardExtras(m.client, m.workspaceSlug, m.project.ID))
	}
	if m.screen == screenDetail && m.detailItem != nil {
		m.commentsLoading = true
		cmds = append(cmds, fetchComments(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID))
	}
	return m, tea.Batch(cmds...)
}

// shouldSkipRefresh reports whether a background refresh would get in the user's way right
// now: while the board is still loading (or already refreshing), while an editor has focus,
// and while a picker is open — a picker indexes into the very lists a refresh replaces.
func (m Model) shouldSkipRefresh() bool {
	if m.client == nil || m.project.ID == "" {
		return true
	}
	if m.screen != screenBoard && m.screen != screenDetail {
		return true
	}
	if m.loading || m.refreshing || m.itemsLoadingMore {
		return true
	}
	if m.editorOn || m.pickerOpen != "" || m.filterOpen != "" || m.idPromptOpen || m.titleSearchOpen {
		return true
	}
	if m.colorPromptOpen || m.attachPathPromptOpen {
		return true
	}
	return false
}

// handleBoardRefreshed swaps a freshly fetched board in underneath the one on screen. The
// whole board arrives in one message (states and every page of work items), so the swap is
// atomic: nothing is ever cleared and re-filled, which is what would make the board blink
// every 30 seconds. A failed refresh leaves the current board untouched.
func (m Model) handleBoardRefreshed(msg boardRefreshedMsg) (tea.Model, tea.Cmd) {
	m.refreshing = false
	m.status = ""
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	m.applyBoardRefresh(msg.states, msg.items, msg.labels)
	return m, nil
}

// applyBoardRefresh replaces the board's data while keeping everything the user set up
// around it: the focused column and each column's cursor follow their state by ID (not by
// position, so a new or removed state does not shift them), and the filters stay as they are.
func (m *Model) applyBoardRefresh(states []api.State, items []api.WorkItem, labels []api.Label) {
	prevCursor := make(map[string]int, len(m.colCursor))
	for i, st := range m.states {
		if i < len(m.colCursor) {
			prevCursor[st.ID] = m.colCursor[i]
		}
	}
	focusedID := ""
	if m.focusedCol >= 0 && m.focusedCol < len(m.states) {
		focusedID = m.states[m.focusedCol].ID
	}

	m.states = states
	m.items = items
	if len(labels) > 0 {
		m.labels = labels
	}

	m.colCursor = make([]int, len(states))
	for i, st := range states {
		m.colCursor[i] = prevCursor[st.ID]
		if st.ID == focusedID {
			m.focusedCol = i
		}
	}
	m.clampBoardCursors()

	// Keep an open detail screen pointing at the refreshed copy of its work item.
	if m.detailItem != nil {
		for i := range m.items {
			if m.items[i].ID == m.detailItem.ID {
				item := m.items[i]
				m.detailItem = &item
				break
			}
		}
	}
}

// clampBoardCursors pulls the focused column and every card cursor back inside the board
// after its contents changed under them.
func (m *Model) clampBoardCursors() {
	if len(m.colCursor) != len(m.states) {
		resized := make([]int, len(m.states))
		copy(resized, m.colCursor)
		m.colCursor = resized
	}
	if m.focusedCol >= len(m.states) {
		m.focusedCol = len(m.states) - 1
	}
	if m.focusedCol < 0 {
		m.focusedCol = 0
	}
	for i := range m.colCursor {
		n := len(m.columnItems(m.states[i].ID))
		if m.colCursor[i] >= n {
			m.colCursor[i] = n - 1
		}
		if m.colCursor[i] < 0 {
			m.colCursor[i] = 0
		}
	}
}

// handleBoardItemsPage appends a streamed-in page of work items to the board that is
// already on screen, and immediately requests the next page if there is one. This is what
// lets a large project's board fill in progressively instead of blocking on every page
// before showing anything (and timing out entirely for very large projects).
func (m Model) handleBoardItemsPage(msg boardItemsPageMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.itemsLoadingMore = false
		m.setError(msg.err)
		return m, nil
	}
	m.items = append(m.items, msg.items...)
	m.itemsNextCursor = msg.nextCursor
	if msg.hasNext {
		return m, fetchWorkItemsPage(m.client, m.workspaceSlug, m.project.ID, msg.nextCursor)
	}
	m.itemsLoadingMore = false
	return m, nil
}

// columnItems returns the work items in the given state that also pass the active
// assignee/label/state/priority filters (see filter.go) and the title search (m.titleSearch,
// see updateTitleSearch) — a plain case-insensitive substring match against the already-loaded
// m.items, no API call — ordered by the board's sort mode.
func (m Model) columnItems(stateID string) []api.WorkItem {
	q := strings.ToLower(strings.TrimSpace(m.titleSearch))
	var out []api.WorkItem
	for _, it := range m.items {
		if it.State != stateID {
			continue
		}
		if m.filterAssignee != "" && !containsStr(it.Assignees, m.filterAssignee) {
			continue
		}
		if m.filterLabel != "" && !containsStr(it.Labels, m.filterLabel) {
			continue
		}
		if m.filterState != "" && it.State != m.filterState {
			continue
		}
		if m.filterPriority != "" && it.Priority != m.filterPriority {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(it.Name), q) {
			continue
		}
		out = append(out, it)
	}
	sortWorkItems(out, m.sortMode)
	return out
}

// sortModes are the board's card ordering options, in the order the picker lists them. The
// first one is the default: whatever order the API returned the items in.
var sortModes = []struct{ key, label string }{
	{sortDefault, "Default (as the API returns them)"},
	{"priority", "Priority (urgent first)"},
	{"created-desc", "Created (newest first)"},
	{"created-asc", "Created (oldest first)"},
	{"updated-desc", "Recently updated first"},
	{"name", "Name (A-Z)"},
	{"id-asc", "Work item number (ascending)"},
}

const sortDefault = "default"

// normalizeSortMode maps a persisted (or empty) sort mode onto a known one, so an old config
// file — or a hand-edited one — cannot leave the board in an ordering nothing implements.
func normalizeSortMode(mode string) string {
	for _, sm := range sortModes {
		if sm.key == mode {
			return mode
		}
	}
	return sortDefault
}

// sortModeLabel is a sort mode's human-readable name.
func sortModeLabel(mode string) string {
	for _, sm := range sortModes {
		if sm.key == mode {
			return sm.label
		}
	}
	return sortModes[0].label
}

// sortWorkItems orders one column's cards in place. Every comparison is a stable sort, so
// cards that compare equal keep the order the API returned them in. Timestamps are compared
// as strings: Plane returns them all as UTC RFC 3339, where lexical and chronological order
// agree.
func sortWorkItems(items []api.WorkItem, mode string) {
	switch normalizeSortMode(mode) {
	case "priority":
		sort.SliceStable(items, func(i, j int) bool {
			return api.PriorityRank(items[i].Priority) < api.PriorityRank(items[j].Priority)
		})
	case "created-desc":
		sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })
	case "created-asc":
		sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt < items[j].CreatedAt })
	case "updated-desc":
		sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt > items[j].UpdatedAt })
	case "name":
		sort.SliceStable(items, func(i, j int) bool {
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		})
	case "id-asc":
		sort.SliceStable(items, func(i, j int) bool { return items[i].SequenceID < items[j].SequenceID })
	}
}

func containsStr(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func (m Model) selectedItem() *api.WorkItem {
	if len(m.states) == 0 || m.focusedCol < 0 || m.focusedCol >= len(m.states) {
		return nil
	}
	// A collapsed column shows no cards, so it has nothing selected — otherwise s/y/enter
	// would act on a card the user cannot see.
	if m.isHidden(m.states[m.focusedCol].ID) {
		return nil
	}
	if m.focusedCol >= len(m.colCursor) {
		return nil
	}
	col := m.columnItems(m.states[m.focusedCol].ID)
	if len(col) == 0 {
		return nil
	}
	idx := m.colCursor[m.focusedCol]
	if idx < 0 || idx >= len(col) {
		return nil
	}
	return &col[idx]
}

func (m Model) findState(id string) *api.State {
	for i := range m.states {
		if m.states[i].ID == id {
			return &m.states[i]
		}
	}
	return nil
}

func (m Model) updateBoard(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.colorPromptOpen {
		return m.updateColorPrompt(msg)
	}
	if m.pickerOpen != "" {
		return m.updatePicker(msg)
	}
	if m.filterOpen != "" {
		return m.updateFilterPicker(msg)
	}
	if m.editorOn {
		return m.updateEditor(msg)
	}
	if m.creatingItem {
		return m.updateNewItemReview(msg)
	}
	if m.idPromptOpen {
		return m.updateIDPrompt(msg)
	}
	if m.titleSearchOpen {
		return m.updateTitleSearch(msg)
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q":
		return m, tea.Quit
	case "left", "h":
		if m.focusedCol > 0 {
			m.focusedCol--
		}
	case "right", "l":
		if m.focusedCol < len(m.states)-1 {
			m.focusedCol++
		}
	case "up", "k":
		if m.focusedCol < len(m.colCursor) && m.colCursor[m.focusedCol] > 0 {
			m.colCursor[m.focusedCol]--
		}
	case "down", "j":
		if m.focusedCol < len(m.colCursor) {
			col := m.columnItems(m.states[m.focusedCol].ID)
			if m.colCursor[m.focusedCol] < len(col)-1 {
				m.colCursor[m.focusedCol]++
			}
		}
	case "r":
		// A manual refresh takes the same non-blinking path as the automatic one: the board
		// stays on screen and is swapped out once the new data is in.
		if m.refreshing {
			return m, nil
		}
		m.refreshing = true
		m.status = "Refreshing..."
		return m, refreshBoard(m.client, m.workspaceSlug, m.project.ID)
	case "x":
		return m.toggleHiddenColumn()
	case "C":
		m.openColorTargetPicker()
	case "o":
		m.openSortPicker()
	case "n":
		return m.openNewItemEditor()
	case "p":
		m.screen = screenProjects
	case "w":
		return m.switchWorkspace()
	case "enter":
		if item := m.selectedItem(); item != nil {
			// Open on the board's cached copy straight away, then re-fetch the work item
			// and its comments in the background so the card is never shown stale (the
			// footer says a refresh is in flight — see detailLoader). openWorkItem is the
			// same path the detail screen's parent/sub-task jumps take.
			return m.openWorkItem(*item)
		}
	case "s":
		m.openStatePicker()
	case "y":
		m.openPriorityPicker()
	case "A":
		m.openAssigneePicker()
	case "T":
		m.openLabelPicker()
	case "a":
		m.openFilterPicker("assignee")
	case "L":
		m.openFilterPicker("label")
	case "S":
		m.openFilterPicker("state")
	case "Y":
		m.openFilterPicker("priority")
	case "/":
		m.openTitleSearch()
	case "u":
		return m.copyItemURL(m.selectedItem())
	case "g":
		m.openIDPrompt()
	}
	return m, nil
}

// isHidden reports whether a state's column is collapsed to a placeholder on this board.
func (m Model) isHidden(stateID string) bool {
	return m.hiddenStates[stateID]
}

// hiddenColumns is isHidden for every state, in column order — the shape boardLayoutFor and
// viewBoard need.
func (m Model) hiddenColumns() []bool {
	out := make([]bool, len(m.states))
	for i, st := range m.states {
		out[i] = m.isHidden(st.ID)
	}
	return out
}

// hiddenCount is how many of the board's columns are currently collapsed.
func (m Model) hiddenCount() int {
	n := 0
	for _, st := range m.states {
		if m.isHidden(st.ID) {
			n++
		}
	}
	return n
}

// toggleHiddenColumn collapses the focused column to a placeholder (or brings a collapsed one
// back) and persists the choice for this project, so the board reopens the same way.
func (m Model) toggleHiddenColumn() (tea.Model, tea.Cmd) {
	if m.focusedCol < 0 || m.focusedCol >= len(m.states) {
		return m, nil
	}
	id := m.states[m.focusedCol].ID
	hidden := make(map[string]bool, len(m.hiddenStates)+1)
	for k, v := range m.hiddenStates {
		hidden[k] = v
	}
	if hidden[id] {
		delete(hidden, id)
	} else {
		hidden[id] = true
	}
	m.hiddenStates = hidden
	m.cfg.SetHiddenStates(m.project.ID, hidden)
	if err := config.Save(m.cfg); err != nil {
		m.setError(err)
	}
	return m, nil
}

// openNewItemEditor opens the shared textarea to compose a new work item's title. The item
// defaults to the currently focused column's state and no priority/assignee — all three can
// still be changed on the review step that follows (see updateNewItemReview) before it is
// actually created.
func (m Model) openNewItemEditor() (tea.Model, tea.Cmd) {
	if m.focusedCol < 0 || m.focusedCol >= len(m.states) {
		return m, nil
	}
	m.openEditor("new-item", "New work item title...", "", 3)
	m.creatingItem = true
	m.newItemStateID = m.states[m.focusedCol].ID
	m.newItemPriority = "none"
	m.newItemAssignee = ""
	m.newItemLabels = nil
	return m, nil
}

// resetNewItem clears everything openNewItemEditor set up, whether creation finished
// (handleWorkItemCreated) or the user backed out of it (esc, on the title editor or the
// review step).
func (m *Model) resetNewItem() {
	m.creatingItem = false
	m.newItemName = ""
	m.newItemStateID = ""
	m.newItemPriority = ""
	m.newItemAssignee = ""
	m.newItemLabels = nil
}

// updateNewItemReview drives the board's new-work-item review step: once a title has been
// typed (openNewItemEditor's editor, confirmed with enter — see updateEditor), the user lands
// here and can still change the state/priority/assignee/labels it will be created with, the
// same s/y/T keys the board and detail screens already use, plus a on this screen for the
// assignee. enter fires the actual POST; esc cancels the whole thing.
func (m Model) updateNewItemReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc":
		m.resetNewItem()
	case "s":
		m.openNewItemStatePicker()
	case "y":
		m.openNewItemPriorityPicker()
	case "a":
		m.openNewItemAssigneePicker()
	case "T":
		m.openNewItemLabelPicker()
	case "enter":
		fields := map[string]any{
			"name":     m.newItemName,
			"state":    m.newItemStateID,
			"priority": m.newItemPriority,
		}
		if m.newItemAssignee != "" {
			fields["assignees"] = []string{m.newItemAssignee}
		}
		if len(m.newItemLabels) > 0 {
			fields["labels"] = m.newItemLabels
		}
		m.status = "Creating work item..."
		return m, createWorkItem(m.client, m.workspaceSlug, m.project.ID, fields)
	}
	return m, nil
}

// handleWorkItemCreated applies the result of creating a work item from the board: on success
// the new item is appended to m.items and the focused column's cursor moved onto it, the same
// way handleCommentSaved updates local state after a create. The cursor is set to the new
// item's actual position in columnItems' output rather than assumed to be last — columnItems
// sorts (m.sortMode) and filters (assignee/label/state/priority filters, title search), any of
// which can put the new item anywhere in the column, or filter it out of view entirely, in
// which case the cursor is left at the top of the column instead of pointing past its end.
func (m Model) handleWorkItemCreated(msg workItemCreatedMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	m.closeEditor()
	m.resetNewItem()
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	m.items = append(m.items, *msg.item)
	for i, st := range m.states {
		if st.ID != msg.item.State {
			continue
		}
		m.focusedCol = i
		if i < len(m.colCursor) {
			idx := 0
			for j, it := range m.columnItems(st.ID) {
				if it.ID == msg.item.ID {
					idx = j
					break
				}
			}
			m.colCursor[i] = idx
		}
		break
	}
	return m, nil
}

// viewNewItemEditor renders the board's new-work-item title composer, in the same style
// detailBottom uses for the comment/description editor.
func (m Model) viewNewItemEditor() string {
	return focusedInputStyle.Render(columnHeaderStyle.Render("New work item") + "\n" + m.editor.View() + "\n" +
		helpStyle.Render("enter  next    alt+enter/ctrl+j  new line    esc  cancel"))
}

// viewNewItemReview renders the review step that follows the title editor: the item's name
// plus the state/priority/assignee it will be created with, each changeable before enter
// fires the actual create.
func (m Model) viewNewItemReview() string {
	lines := []string{
		"Name:      " + m.newItemName,
		"State:     " + m.stateName(m.newItemStateID),
		"Priority:  " + m.priorityLabel(m.newItemPriority),
		"Assignee:  " + m.newItemAssigneeName(),
		"Labels:    " + m.labelsLine(m.newItemLabels),
	}
	body := columnHeaderStyle.Render("New work item") + "\n" + strings.Join(lines, "\n") + "\n" +
		helpStyle.Render("s  state    y  priority    a  assignee    T  labels    enter  create    esc  cancel")
	return focusedInputStyle.Render(body)
}

// newItemAssigneeName is the review step's "Assignee:" value.
func (m Model) newItemAssigneeName() string {
	if m.newItemAssignee == "" {
		return "Unassigned"
	}
	return m.memberName(m.newItemAssignee)
}

// openIDPrompt opens the board's "open by work item id" prompt (g): the user types a work
// item's number and enter opens it, the same way pressing enter on its card does.
func (m *Model) openIDPrompt() {
	m.setError(nil)
	m.idInput.Reset()
	m.idInput.Focus()
	m.idPromptOpen = true
}

// closeIDPrompt puts the id prompt away without opening anything.
func (m Model) closeIDPrompt() Model {
	m.idPromptOpen = false
	m.idInput.Blur()
	m.idInput.Reset()
	return m
}

// updateIDPrompt drives the board's "open by work item id" prompt. There is no by-sequence-
// number lookup in Plane's public API (the work-items endpoint is only keyed by its own UUID),
// so this can only find a work item the board has already loaded into m.items — on a large
// project still streaming its later pages in, a very recently created item may not be there
// yet.
func (m Model) updateIDPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m.closeIDPrompt(), nil
		case "enter":
			raw := strings.TrimSpace(m.idInput.Value())
			n, err := strconv.Atoi(raw)
			if err != nil || n <= 0 {
				m.setError(fmt.Errorf("enter a work item number"))
				return m, nil
			}
			for _, it := range m.items {
				if it.SequenceID == n {
					m = m.closeIDPrompt()
					return m.openWorkItem(it)
				}
			}
			m.setError(fmt.Errorf("work item #%d is not loaded on this board", n))
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.idInput, cmd = m.idInput.Update(msg)
	return m, cmd
}

// viewIDPrompt renders the board's "open by work item id" prompt.
func (m Model) viewIDPrompt() string {
	body := columnHeaderStyle.Render("Open work item by id") + "\n" + m.idInput.View() + "\n" +
		helpStyle.Render("enter  open    esc  cancel")
	return focusedInputStyle.Render(body)
}

// openTitleSearch opens the board's "/" title search box, pre-filled with whatever query is
// already applied (so re-opening it to tweak a search does not lose it).
func (m *Model) openTitleSearch() {
	m.titleSearchInput.SetValue(m.titleSearch)
	m.titleSearchInput.CursorEnd()
	m.titleSearchInput.Focus()
	m.titleSearchOpen = true
}

// updateTitleSearch drives the board's title search box: typing narrows every column, live, to
// cards whose title contains the query (case-insensitive — see columnItems); enter keeps the
// query applied and returns to board navigation, esc clears it and does the same. Like every
// board filter this makes no API call of its own — it only narrows m.items, which the board
// already has loaded ("simple, no api calls, only on loaded tasks").
func (m Model) updateTitleSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.titleSearch = ""
			m.titleSearchInput.Reset()
			m.titleSearchInput.Blur()
			m.titleSearchOpen = false
			m.clampBoardCursors()
			return m, nil
		case "enter":
			m.titleSearchInput.Blur()
			m.titleSearchOpen = false
			m.clampBoardCursors()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.titleSearchInput, cmd = m.titleSearchInput.Update(msg)
	m.titleSearch = m.titleSearchInput.Value()
	// The query just narrowed (or widened) every column: pull each card cursor back inside its
	// column the same way clampBoardCursors already does after a background refresh.
	m.clampBoardCursors()
	return m, cmd
}

// viewTitleSearch renders the board's title search box.
func (m Model) viewTitleSearch() string {
	body := columnHeaderStyle.Render("Search titles") + "\n" + m.titleSearchInput.View() + "\n" +
		helpStyle.Render("type  filter cards    enter  keep    esc  clear")
	return focusedInputStyle.Render(body)
}

func (m Model) viewBoard() string {
	if m.loading {
		return boardTitleStyle.Render(m.project.Name) + "\n\nLoading board...\n\n" + m.footer("")
	}
	if len(m.states) == 0 {
		return boardTitleStyle.Render(m.project.Name) + "\n\n" + helpStyle.Render("This project has no states.") + "\n\n" + m.footer(helpStyle.Render("p  switch board    q  quit"))
	}

	// Every card carries a sub-task badge, so count each item's children once here rather
	// than letting all ~maxRows*visibleCols cards rescan the project's item list (see
	// subIssueCount). m is a value receiver, so this only lives for the length of the frame.
	m.subCounts = m.childCounts()

	width, height := m.termSize()
	hidden := m.hiddenColumns()
	colWidth, firstCol, visibleCols := boardLayoutFor(width, hidden, m.focusedCol)

	header := boardTitleStyle.Render(m.project.Name)
	if f := m.activeFilterSummary(); f != "" {
		header += "  " + helpStyle.Render(f)
	}
	if m.titleSearch != "" {
		header += "  " + helpStyle.Render(fmt.Sprintf("search: %q", m.titleSearch))
	}
	if m.sortMode != "" && m.sortMode != sortDefault {
		header += "  " + helpStyle.Render("sorted by "+sortModeLabel(m.sortMode))
	}
	if n := m.hiddenCount(); n > 0 {
		header += "  " + helpStyle.Render(fmt.Sprintf("%d hidden", n))
	}
	if m.itemsLoadingMore {
		header += "  " + helpStyle.Render(fmt.Sprintf("loading more… (%d so far)", len(m.items)))
	}
	if m.refreshing {
		header += "  " + helpStyle.Render("refreshing…")
	}
	if visibleCols < len(m.states) {
		header += "  " + helpStyle.Render(fmt.Sprintf("columns %d-%d of %d", firstCol+1, firstCol+visibleCols, len(m.states)))
	}

	header = clampLines(header, width)
	footer := clampLines(m.footer(packHints(boardHints, width)), width)

	// Overlays are appended below the footer, so they eat into the rows the columns may use.
	var overlay string
	if m.pickerOpen != "" {
		overlay = "\n\n" + m.viewPicker()
	}
	if m.filterOpen != "" {
		overlay += "\n\n" + m.viewFilterPicker()
	}
	if m.editorOn {
		overlay += "\n\n" + m.viewNewItemEditor()
	}
	if m.creatingItem && m.pickerOpen == "" && !m.editorOn {
		overlay += "\n\n" + m.viewNewItemReview()
	}
	if m.idPromptOpen {
		overlay += "\n\n" + m.viewIDPrompt()
	}
	if m.titleSearchOpen {
		overlay += "\n\n" + m.viewTitleSearch()
	}
	if m.colorPromptOpen {
		overlay += "\n\n" + m.viewColorPrompt()
	}

	// Everything on screen that is not a card row: the header and its blank line, the blank
	// line above the footer, the footer itself, any overlay, and each column's own top
	// border, title row and bottom border (colChromeRows).
	chrome := func() int {
		n := visualHeight(header, width) + 1 + 1 + visualHeight(footer, width) + colChromeRows
		if overlay != "" {
			n += visualHeight(overlay, width)
		}
		return n
	}
	// On a terminal too short to show the key hints and a usable column both, the hints go:
	// they wrap onto several rows at that width, and the board is the part worth keeping.
	if height-chrome() < minCardRows {
		footer = clampLines(m.footer(""), width)
	}
	maxRows := boardCardRows(height, chrome())

	var cols []string
	for ci := firstCol; ci < firstCol+visibleCols; ci++ {
		if hidden[ci] {
			cols = append(cols, m.renderCollapsedColumn(ci, maxRows))
			continue
		}
		cols = append(cols, m.renderColumn(ci, colWidth, maxRows))
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols...)

	return clipRows(header+"\n\n"+body+"\n\n"+footer+overlay, height)
}

// clipRows drops any rows past the terminal's height. The layout arithmetic above already
// sizes the board to fit, so this only bites on a terminal too small for even minCardRows
// cards — where dropping the bottom of the frame still beats overflowing, which scrolls the
// top of the board (its title and every column header) off the screen for good.
func clipRows(s string, height int) string {
	if height <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= height {
		return s
	}
	return strings.Join(lines[:height], "\n")
}

// termSize resolves the terminal size to lay the board out in, falling back to
// defaultTerminalWidth/defaultTerminalHeight while the real size is still unknown.
func (m Model) termSize() (width, height int) {
	width, height = m.width, m.height
	if width <= 0 {
		width = defaultTerminalWidth
	}
	if height <= 0 {
		height = defaultTerminalHeight
	}
	return width, height
}

// boardLayout decides how the board's columns are laid out in a terminal `width` columns wide:
// the width of one column, the index of the leftmost column shown, and how many are shown. It
// is boardLayoutFor for a board with no collapsed columns.
func boardLayout(width, columns, focused int) (colWidth, first, visible int) {
	return boardLayoutFor(width, make([]bool, columns), focused)
}

// boardLayoutFor is boardLayout for a board where hidden[i] reports whether column i is
// collapsed to a placeholder: a collapsed column costs hiddenColWidth+colFrame terminal
// columns instead of colWidth+colFrame, which is the point of collapsing one — the width it
// gives up is handed to the columns still expanded.
//
// The layout always fits: the columns it chooses to show never need more than `width`
// terminal columns between them. When the board cannot show every state at minColWidth it
// shows the widest window of columns that does fit, centred on the focused one, rather than
// rendering them all and letting the terminal wrap the board into an unreadable mess.
func boardLayoutFor(width int, hidden []bool, focused int) (colWidth, first, visible int) {
	columns := len(hidden)
	if columns <= 0 {
		return 0, 0, 0
	}
	if width < minColWidth+colFrame {
		width = minColWidth + colFrame
	}
	if focused < 0 {
		focused = 0
	}
	if focused >= columns {
		focused = columns - 1
	}

	// fit returns the widest an expanded column may be so that the window [first, first+count)
	// fits in width, or ok=false if it cannot fit even at minColWidth.
	fit := func(first, count int) (int, bool) {
		expanded, fixed := 0, 0
		for i := first; i < first+count; i++ {
			if hidden[i] {
				fixed += hiddenColWidth + colFrame
				continue
			}
			expanded++
			fixed += colFrame
		}
		if expanded == 0 {
			// Every column in the window is collapsed: nothing to size, it either fits or not.
			return minColWidth, fixed <= width
		}
		w := (width - fixed) / expanded
		if w > maxColWidth {
			w = maxColWidth
		}
		if w < minColWidth {
			return 0, false
		}
		return w, true
	}

	for count := columns; count >= 1; count-- {
		first := focused - (count-1)/2
		if first+count > columns {
			first = columns - count
		}
		if first < 0 {
			first = 0
		}
		if w, ok := fit(first, count); ok {
			return w, first, count
		}
	}
	// Not even one column fits at minColWidth (a terminal narrower than a single column):
	// give the whole terminal to the focused one.
	return width - colFrame, focused, 1
}

// boardCardRows is how many cards one column may render, given the terminal height and the
// rows the board's chrome already claims (each card costs cardRows rows). It never returns
// less than minCardRows: on a terminal too short for even that the board is clipped by the
// terminal anyway, and returning 0 would hide the selected card entirely.
func boardCardRows(height, chrome int) int {
	if cards := (height - chrome) / cardRows; cards > minCardRows {
		return cards
	}
	return minCardRows
}

// visualHeight is how many terminal rows s occupies once the terminal wraps any line wider
// than width — unlike lipgloss.Height, which only counts the newlines actually in s and so
// under-counts a long single-line footer on a narrow terminal.
func visualHeight(s string, width int) int {
	if width <= 0 {
		return lipgloss.Height(s)
	}
	rows := 0
	for _, line := range strings.Split(s, "\n") {
		n := (lipgloss.Width(line) + width - 1) / width
		if n < 1 {
			n = 1
		}
		rows += n
	}
	return rows
}

// clampLines clips every line of s to at most width terminal columns, so a long project name,
// filter summary or error message overflows into an ellipsis instead of wrapping onto a row
// the board never budgeted for (and pushing the bottom of every column off the screen).
func clampLines(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > width {
			lines[i] = truncate(line, width)
		}
	}
	return strings.Join(lines, "\n")
}

// boardHints are the board footer's key hints, kept as separate chunks so packHints can wrap
// them at a word boundary instead of letting the terminal split one mid-hint.
var boardHints = [][2]string{
	{"h/l", "column"}, {"j/k", "card"}, {"enter", "open"}, {"s", "state"}, {"y", "priority"},
	{"A", "assignee"}, {"T", "labels"}, {"a", "filter assignee"}, {"L", "filter label"},
	{"S", "filter state"}, {"Y", "filter priority"}, {"/", "search title"}, {"u", "copy url"},
	{"g", "open by id"}, {"o", "order"}, {"n", "new item"}, {"x", "hide col"}, {"C", "colors"},
	{"r", "refresh"}, {"p", "boards"}, {"q", "quit"},
}

// packHints joins hint key/description pairs into as few lines as fit within width, styling
// each key in helpKeyStyle and its description (plus every separator) in helpStyle — a
// shortcut used to read in exactly the same dim color as the text describing it, with nothing
// to make it stand out as the part to actually press. The full hint line is 131 columns wide,
// so on anything narrower the terminal used to wrap it — silently costing the board a row it
// had not budgeted for, and pushing the bottom of the last column off screen. Line-breaking is
// still decided from each pair's plain "key  desc" width (the same math as before this had any
// per-part styling), never the styled one, so the ANSI escape bytes styling adds can never skew
// where a line actually breaks.
func packHints(hints [][2]string, width int) string {
	const sep = "    "
	plain := func(h [2]string) string { return h[0] + "  " + h[1] }
	styled := func(h [2]string) string { return helpKeyStyle.Render(h[0]) + helpStyle.Render("  "+h[1]) }

	var lines []string
	var curPlain, curStyled string
	for _, h := range hints {
		p := plain(h)
		switch {
		case curPlain == "":
			curPlain, curStyled = p, styled(h)
		case len(curPlain)+len(sep)+len(p) <= width:
			curPlain += sep + p
			curStyled += helpStyle.Render(sep) + styled(h)
		default:
			lines = append(lines, curStyled)
			curPlain, curStyled = p, styled(h)
		}
	}
	if curPlain != "" {
		lines = append(lines, curStyled)
	}
	return strings.Join(lines, "\n")
}

// Board layout constants. Every one of them is in terminal columns/rows, and they exist
// because the rendered size of a column or a card is *not* the width it is asked for: the
// card padding is inside cardStyle.Width, so ignoring it is what made every card with a long
// title wrap onto a second line, which doubled the height of a full-screen board and pushed
// the bottom of every column off the screen.
const (
	// minColWidth is the narrowest a column may be squeezed to before the board shows a
	// window of columns instead of all of them; maxColWidth is the widest it grows to.
	minColWidth = 20
	maxColWidth = 40

	// colFrame is what a column costs beyond its own Width. columnStyle has no border, so
	// this is 0 — a column of colWidth occupies exactly colWidth terminal columns. It stays
	// a named constant (rather than being dropped) because the layout math below still adds
	// it wherever a border-ful widget would have cost extra, so a future border does not
	// require re-deriving that arithmetic.
	colFrame = 0

	// hiddenColWidth is the content width of a collapsed column's placeholder: just wide
	// enough for one character of its vertically stacked name, so hiding a column hands
	// nearly all of its width to the columns that are still expanded.
	hiddenColWidth = 3

	// colPadding is columnStyle's own horizontal padding: cards inside a column of colWidth
	// get colWidth-colPadding to render in.
	colPadding = 2

	// cardFrame is cardStyle's horizontal padding: a card rendered at width w wraps onto a
	// second line as soon as its text is wider than w-cardFrame.
	cardFrame = 2

	// cardRows is how many terminal rows one card occupies: the title wrapped across up to
	// two lines, plus one line for its labels and priority.
	cardRows = 3

	// colChromeRows is what a column costs vertically besides its cards: just its own header
	// row, now that columnStyle has no border.
	colChromeRows = 1

	// minCardRows is the smallest number of cards a column is ever given room for. A
	// terminal too short for even this clips the board itself, but showing zero cards would
	// hide the cursor.
	minCardRows = 3
)

// defaultTerminalHeight is the row-clipping fallback used before the first WindowSizeMsg
// has arrived (or if the terminal never reports one at all). Without it, a board rendered
// before m.height is known skips clipping entirely: on a project with a big list of tasks
// this produces a frame thousands of lines tall that no terminal, full screen or otherwise,
// can actually show.
const defaultTerminalHeight = 24

// defaultTerminalWidth is the same kind of fallback as defaultTerminalHeight, for the
// horizontal dimension: on a terminal that never reports its size, m.width stays 0, which
// must not be read as "plenty of room" — that skipped the narrow-terminal layout entirely and
// rendered every column at full width side by side, producing lines far wider than the real
// (unknown) terminal and wrapping the whole board into an unreadable mess.
const defaultTerminalWidth = 80

// renderColumn renders a single kanban column at the given width, showing at most maxRows
// cards around the column's cursor. Every card it renders is exactly cardRows rows tall — its
// title wraps onto at most 2 lines and its labels/priority render on a 3rd — so the column is
// exactly maxRows*cardRows+colChromeRows rows tall and the caller can budget the board against
// the terminal height.
func (m Model) renderColumn(ci, colWidth, maxRows int) string {
	st := m.states[ci]

	items := m.columnItems(st.ID)
	cursor := 0
	if ci < len(m.colCursor) {
		cursor = m.colCursor[ci]
	}
	visible := items
	scrolled := false

	if maxRows < minCardRows {
		maxRows = minCardRows
	}
	if len(items) > maxRows {
		start := cursor - maxRows/2
		if start+maxRows > len(items) {
			start = len(items) - maxRows
		}
		if start < 0 {
			start = 0
		}
		visible = items[start : start+maxRows]
		cursor -= start
		scrolled = true
	}

	cardWidth := colWidth - colPadding
	if cardWidth < 1 {
		cardWidth = 1
	}

	headerText := fmt.Sprintf("%s (%d)", st.Name, len(items))
	if scrolled {
		headerText += " *"
	}
	// The header is tinted with the state's own (pastelized) color so a column reads as
	// which state it is at a glance; the focused column additionally gets an underline,
	// rather than losing its color to swap to the plain focus style.
	headerStyle := columnHeaderStyle.Foreground(pastelStateColor(m.effectiveStateColor(st)))
	if ci == m.focusedCol {
		headerStyle = headerStyle.Underline(true)
	}
	out := headerStyle.Render(truncate(headerText, cardWidth)) + "\n"
	for ii, it := range visible {
		selected := ci == m.focusedCol && ii == cursor
		block := strings.Join(m.cardLines(it, cardWidth, selected), "\n")
		if selected {
			out += cardSelectedStyle.Width(cardWidth).Render(block) + "\n"
		} else {
			out += cardStyle.Width(cardWidth).Render(block) + "\n"
		}
	}
	return columnStyle.Width(colWidth).Render(strings.TrimRight(out, "\n"))
}

// renderCollapsedColumn renders a hidden state as a narrow placeholder: its name and card
// count stacked one character per row, so the column still says which state it is (and that
// x brings it back) while costing the board only hiddenColWidth+colFrame terminal columns.
// Like renderColumn it never grows past maxRows*cardRows+colChromeRows rows — the same total
// height an expanded column of maxRows cards would take, so every column in a row lines up.
func (m Model) renderCollapsedColumn(ci, maxRows int) string {
	st := m.states[ci]
	// A plain style, not hiddenColumnStyle itself: that one carries columnStyle's horizontal
	// padding, which would widen just this one row beyond hiddenColWidth and wrap it onto an
	// extra line once the whole block is re-rendered at that width below.
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorHiddenFg)
	if ci == m.focusedCol {
		headerStyle = headerStyle.Underline(true)
	}
	if maxRows < minCardRows {
		maxRows = minCardRows
	}
	budget := maxRows * cardRows

	label := []rune(fmt.Sprintf("%s %d", st.Name, len(m.columnItems(st.ID))))
	rows := []string{headerStyle.Render(cell("▸", hiddenColWidth))}
	for _, r := range label {
		if len(rows)-1 >= budget {
			// Out of room: mark the label as clipped rather than silently dropping the rest.
			rows[len(rows)-1] = cell("…", hiddenColWidth)
			break
		}
		rows = append(rows, cell(string(r), hiddenColWidth))
	}
	// hiddenColumnStyle's own filled-in background (colorHiddenBg) is what marks this column
	// as collapsed, rather than just its reduced width — see the task that asked for it.
	return hiddenColumnStyle.Width(hiddenColWidth).Render(strings.Join(rows, "\n"))
}

// cell renders s in exactly width terminal columns, truncating or right-padding it so a
// collapsed column's rows all line up inside its border.
func cell(s string, width int) string {
	s = truncate(s, width)
	if pad := width - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// cardLines is one card's text, sized to render on exactly cardRows rows inside a card of
// cardWidth: the "#<id> <title>" prefix and title wrapped across the first two, clipped with
// an ellipsis if the title still does not fit, and its labels/priority on the third.
//
// selected is true for the card under the cursor, which renderColumn goes on to wrap in
// cardSelectedStyle's background. A lipgloss Style.Render call always ends its output with a
// reset escape sequence, and that reset is not scoped to the styled substring — it turns off
// every attribute for whatever comes after it on the same terminal line. cardNumberStyle here
// and priorityLabel/labelText in cardMetaLine each make such a call, so nesting them inside a
// selected card silently cancelled the outer highlight partway through line1 and line3 (the
// card number and title read normally, one background color; the labels a different, unhighlighted
// one). selected skips that inner coloring so the outer style's background/foreground carries
// the whole line uninterrupted — the per-segment colors are what selection is trading away.
func (m Model) cardLines(it api.WorkItem, cardWidth int, selected bool) []string {
	avail := cardWidth - cardFrame
	if avail < 1 {
		avail = 1
	}
	plainPrefix := fmt.Sprintf("#%d ", it.SequenceID)
	title := oneLine(it.Name)

	// wrapLine cuts the plain (unstyled) "#123 title" text so the wrap point is decided by
	// what is actually on screen; only afterwards is cardNumberStyle applied to however much
	// of plainPrefix survived onto line1 — on a card too narrow to fit even the number, that
	// is less than the whole prefix, so the split has to measure the real cut rather than
	// assuming line1 starts with plainPrefix in full.
	line1, rest := wrapLine(plainPrefix+title, avail)
	if !selected {
		prefixLen := len(plainPrefix)
		if prefixLen > len(line1) {
			prefixLen = len(line1)
		}
		line1 = cardNumberStyle.Render(line1[:prefixLen]) + line1[prefixLen:]
	}
	line2 := truncate(strings.TrimSpace(rest), avail)
	line3 := truncate(m.cardMetaLine(it, selected), avail)

	return []string{line1, line2, line3}
}

// wrapLine splits s at the last space that keeps the first part within width terminal
// columns (or, if there is none, hard-cuts it at width), returning that first part and
// whatever of s did not fit. Used to wrap a card's title onto a second line instead of
// truncating it outright.
func wrapLine(s string, width int) (first, rest string) {
	if lipgloss.Width(s) <= width {
		return s, ""
	}
	cut := ansi.Truncate(s, width, "")
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		cut = cut[:idx]
	}
	return cut, s[len(cut):]
}

// cardMetaLine is a card's third line: its priority (if set), its parent/sub-task badge and
// its label names, resolved against the board's label list.
//
// The relations badge goes here rather than on a row of its own so that a card still costs
// exactly cardRows rows — a fourth row would take a quarter of the cards off every column.
// It sits ahead of the labels because it is structural: when the line has to be truncated,
// what gets cut is the label list rather than the fact that the card has sub-tasks.
func (m Model) cardMetaLine(it api.WorkItem, selected bool) string {
	var parts []string
	if it.Priority != "" && it.Priority != "none" {
		if selected {
			parts = append(parts, priorityText(it.Priority))
		} else {
			parts = append(parts, m.priorityLabel(it.Priority))
		}
	}
	if rel := m.cardRelations(it); rel != "" {
		parts = append(parts, rel)
	}
	if names := m.labelNames(it.Labels, selected); len(names) > 0 {
		parts = append(parts, strings.Join(names, ", "))
	}
	return strings.Join(parts, "  ")
}

// labelNames resolves label IDs to their names against the board's label list, dropping any ID
// the board does not (yet) know about rather than showing a raw UUID. Each name is rendered in
// that label's own color (see labelText) unless selected — see cardLines on why a selected
// card's own background/foreground must not be interrupted by a nested style's reset.
func (m Model) labelNames(ids []string, selected bool) []string {
	if len(ids) == 0 {
		return nil
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		for _, l := range m.labels {
			if l.ID == id {
				if selected {
					names = append(names, l.Name)
				} else {
					names = append(names, labelText(l.Name, m.effectiveLabelColor(l)))
				}
				break
			}
		}
	}
	return names
}

// oneLine flattens a work item name onto a single line. A name containing a newline or a tab
// would otherwise break its card out of its row and misalign every column beside it.
func oneLine(s string) string {
	if !strings.ContainsAny(s, "\n\r\t") {
		return s
	}
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\t'
	}), " ")
}

// truncate clips s to at most n terminal columns, marking a clipped string with an ellipsis.
// It measures display width rather than bytes: slicing by byte length cut multi-byte runes in
// half (a Cyrillic task name rendered a replacement character mid-word) and clipped non-ASCII
// titles two to three times shorter than the space actually available to them.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return ansi.Truncate(s, n, "…")
}
