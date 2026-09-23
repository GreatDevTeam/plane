package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
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
	m.labels = msg.labels
	m.members = msg.members
	m.focusedCol = 0
	m.colCursor = make([]int, len(m.states))
	m.filterAssignee = ""
	m.filterLabel = ""
	m.screen = screenBoard
	if msg.hasNextPage {
		m.itemsLoadingMore = true
		return m, fetchWorkItemsPage(m.client, m.workspaceSlug, m.project.ID, msg.nextCursor)
	}
	return m, nil
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
// assignee/label filters (see filter.go), in a stable order.
func (m Model) columnItems(stateID string) []api.WorkItem {
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
		out = append(out, it)
	}
	return out
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
	if len(m.states) == 0 || m.focusedCol >= len(m.states) {
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
	if m.pickerOpen != "" {
		return m.updatePicker(msg)
	}
	if m.filterOpen != "" {
		return m.updateFilterPicker(msg)
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
		if m.colCursor[m.focusedCol] > 0 {
			m.colCursor[m.focusedCol]--
		}
	case "down", "j":
		col := m.columnItems(m.states[m.focusedCol].ID)
		if m.colCursor[m.focusedCol] < len(col)-1 {
			m.colCursor[m.focusedCol]++
		}
	case "r":
		m.loading = true
		m.status = "Refreshing..."
		return m, fetchBoard(m.client, m.workspaceSlug, m.project.ID)
	case "p":
		m.screen = screenProjects
	case "w":
		return m.switchWorkspace()
	case "enter":
		if item := m.selectedItem(); item != nil {
			m.detailItem = item
			m.comments = nil
			m.commentCursor = 0
			m.commentsLoading = true
			m.screen = screenDetail
			return m, fetchComments(m.client, m.workspaceSlug, m.project.ID, item.ID)
		}
	case "s":
		m.openStatePicker()
	case "y":
		m.openPriorityPicker()
	case "a":
		m.openFilterPicker("assignee")
	case "L":
		m.openFilterPicker("label")
	}
	return m, nil
}

func (m Model) viewBoard() string {
	if m.loading {
		return titleStyle.Render(m.project.Name) + "\n\nLoading board...\n\n" + m.footer("")
	}
	if len(m.states) == 0 {
		return titleStyle.Render(m.project.Name) + "\n\n" + helpStyle.Render("This project has no states.") + "\n\n" + m.footer("p  switch board    q  quit")
	}

	width, height := m.termSize()
	colWidth, firstCol, visibleCols := boardLayout(width, len(m.states), m.focusedCol)

	header := titleStyle.Render(m.project.Name)
	if f := m.activeFilterSummary(); f != "" {
		header += "  " + helpStyle.Render(f)
	}
	if m.itemsLoadingMore {
		header += "  " + helpStyle.Render(fmt.Sprintf("loading more… (%d so far)", len(m.items)))
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
// the width of one column, the index of the leftmost column shown, and how many are shown.
//
// Every returned column costs colWidth+colFrame terminal columns, so the layout always
// satisfies visibleCols*(colWidth+colFrame) <= width. When the states do not all fit at
// minColWidth, the board shows the widest window of columns that does fit, centred on the
// focused one, rather than rendering them all and letting the terminal wrap the board into an
// unreadable mess (which is what a full-screen board used to do).
func boardLayout(width, columns, focused int) (colWidth, first, visible int) {
	if columns <= 0 {
		return 0, 0, 0
	}
	if width < minColWidth+colFrame {
		width = minColWidth + colFrame
	}

	if w := width/columns - colFrame; w >= minColWidth {
		return min(w, maxColWidth), 0, columns
	}

	visible = width / (minColWidth + colFrame)
	if visible < 1 {
		visible = 1
	}
	if visible > columns {
		visible = columns
	}
	if visible == 1 {
		// A single column gets the whole terminal — the layout a half-screen terminal
		// already fell into, which reads fine.
		colWidth = width - colFrame
	} else {
		colWidth = min(width/visible-colFrame, maxColWidth)
	}

	first = focused - (visible-1)/2
	if first+visible > columns {
		first = columns - visible
	}
	if first < 0 {
		first = 0
	}
	return colWidth, first, visible
}

// boardCardRows is how many card rows one column may render, given the terminal height and
// the rows the board's chrome already claims. It never returns less than minCardRows: on a
// terminal too short for even that the board is clipped by the terminal anyway, and returning
// 0 would hide the selected card entirely.
func boardCardRows(height, chrome int) int {
	if rows := height - chrome; rows > minCardRows {
		return rows
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
var boardHints = []string{
	"h/l  column", "j/k  card", "enter  open", "s  state", "y  priority",
	"a  assignee", "L  label", "r  refresh", "p  boards", "q  quit",
}

// packHints joins hints into as few lines as fit within width. The full hint string is 131
// columns wide, so on anything narrower the terminal used to wrap it — silently costing the
// board a row it had not budgeted for, and pushing the bottom of the last column off screen.
func packHints(hints []string, width int) string {
	const sep = "    "
	var lines []string
	cur := ""
	for _, h := range hints {
		switch {
		case cur == "":
			cur = h
		case len(cur)+len(sep)+len(h) <= width:
			cur += sep + h
		default:
			lines = append(lines, cur)
			cur = h
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return strings.Join(lines, "\n")
}

// Board layout constants. Every one of them is in terminal columns/rows, and they exist
// because the rendered size of a column or a card is *not* the width it is asked for:
// lipgloss adds the column border outside columnStyle.Width, and the card padding inside
// cardStyle.Width. Ignoring the latter is what made every card with a long title wrap onto a
// second line, which doubled the height of a full-screen board and pushed the bottom of every
// column off the screen.
const (
	// minColWidth is the narrowest a column may be squeezed to before the board shows a
	// window of columns instead of all of them; maxColWidth is the widest it grows to.
	minColWidth = 20
	maxColWidth = 40

	// colFrame is what columnStyle's rounded border costs on top of the column's width, so a
	// column of colWidth occupies colWidth+colFrame terminal columns.
	colFrame = 2

	// colPadding is columnStyle's own horizontal padding: cards inside a column of colWidth
	// get colWidth-colPadding to render in.
	colPadding = 2

	// cardFrame is cardStyle's horizontal padding: a card rendered at width w wraps onto a
	// second line as soon as its text is wider than w-cardFrame.
	cardFrame = 2

	// colChromeRows is what a column costs vertically besides its cards: its top and bottom
	// border rows plus its own title row.
	colChromeRows = 3

	// minCardRows is the smallest card window a column is ever given. A terminal too short
	// for even this clips the board itself, but showing zero cards would hide the cursor.
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
// cards around the column's cursor. Every card it renders is exactly one row tall — its text
// is truncated to what fits inside both the column's and the card's padding — so the column is
// exactly maxRows+colChromeRows rows tall and the caller can budget the board against the
// terminal height.
func (m Model) renderColumn(ci, colWidth, maxRows int) string {
	st := m.states[ci]
	style := columnStyle
	if ci == m.focusedCol {
		style = columnFocusedStyle
	}

	items := m.columnItems(st.ID)
	cursor := m.colCursor[ci]
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
	out := columnHeaderStyle.Render(truncate(headerText, cardWidth)) + "\n"
	for ii, it := range visible {
		line := cardLine(it, cardWidth)
		if ci == m.focusedCol && ii == cursor {
			out += cardSelectedStyle.Width(cardWidth).Render(line) + "\n"
		} else {
			out += cardStyle.Width(cardWidth).Render(line) + "\n"
		}
	}
	return style.Width(colWidth).Render(strings.TrimRight(out, "\n"))
}

// cardLine is one card's text, sized to render on a single row inside a card of cardWidth.
func cardLine(it api.WorkItem, cardWidth int) string {
	prefix := fmt.Sprintf("#%d ", it.SequenceID)
	avail := cardWidth - cardFrame - lipgloss.Width(prefix)
	if avail < 1 {
		// No room for a title at all: keep the identifier, clipped if it does not fit either.
		return truncate(prefix, cardWidth-cardFrame)
	}
	return prefix + truncate(oneLine(it.Name), avail)
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
