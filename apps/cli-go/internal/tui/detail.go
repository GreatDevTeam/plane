package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// detailPane identifies which of the detail screen's two independently scrollable panes
// currently has keyboard focus.
type detailPane int

const (
	// detailPaneComments is the default: j/k scroll the comments pane one line at a time,
	// independently of which comment is focused; n/p jump the focus itself to the next/previous
	// comment and scroll it fully into view.
	detailPaneComments detailPane = iota
	// detailPaneDescription: j/k instead scroll the description pane by one line.
	detailPaneDescription
)

// minPaneRows is the fewest rows either detail pane is squeezed to on a short terminal —
// the same floor board.go's boardCardRows keeps for the board's columns.
const minPaneRows = 3

func (m Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.attachPathPromptOpen {
		return m.updateAttachPathPrompt(msg)
	}
	if m.pickerOpen != "" {
		return m.updatePicker(msg)
	}
	if m.editorOn {
		return m.updateEditor(msg)
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q":
		return m, tea.Quit
	case "esc", "backspace":
		m.screen = screenBoard
	case "s":
		m.openStatePicker()
	case "y":
		m.openPriorityPicker()
	case "A":
		m.openAssigneePicker()
	case "T":
		m.openLabelPicker()
	case "u":
		return m.copyItemURL(m.detailItem)
	case "c":
		return m.openCommentEditor("")
	case "d":
		return m.openDescriptionEditor()
	case "g":
		return m.jumpToParent()
	case "S":
		m.openSubIssuePicker()
	case "r":
		return m.refreshDetail()
	case "e":
		if len(m.comments) == 0 || m.commentCursor < 0 || m.commentCursor >= len(m.comments) {
			return m, nil
		}
		cm := m.comments[m.commentCursor]
		if m.user == nil || cm.Actor != m.user.ID {
			m.setError(fmt.Errorf("you can only edit your own comments"))
			return m, nil
		}
		return m.openCommentEditor(cm.ID)
	case "tab":
		if m.detailFocus == detailPaneComments {
			m.detailFocus = detailPaneDescription
		} else {
			m.detailFocus = detailPaneComments
		}
	case "up", "k":
		if m.detailFocus == detailPaneDescription {
			m.scrollDescription(-1)
		} else {
			m.scrollComments(-1)
		}
	case "down", "j":
		if m.detailFocus == detailPaneDescription {
			m.scrollDescription(1)
		} else {
			m.scrollComments(1)
		}
	case "n":
		if m.detailFocus == detailPaneComments && m.commentCursor < len(m.comments)-1 {
			m.commentCursor++
			m.scrollCommentsToCursor()
		}
	case "p":
		if m.detailFocus == detailPaneComments && m.commentCursor > 0 {
			m.commentCursor--
			m.scrollCommentsToCursor()
		}
	case "o":
		return m.openFocusedLinks()
	case "f":
		return m.openAttachmentsPicker()
	}
	return m, nil
}

// refreshDetail is the detail screen's manual refresh ("r"): the same board-plus-comments
// refresh the 30s auto-refresh runs (see handleBoardTick), just triggered immediately instead
// of waiting for the next tick.
func (m Model) refreshDetail() (tea.Model, tea.Cmd) {
	if m.refreshing || m.detailItem == nil {
		return m, nil
	}
	m.refreshing = true
	m.commentsLoading = true
	m.attachmentsLoading = true
	m.status = "Refreshing..."
	return m, tea.Batch(
		refreshBoard(m.client, m.workspaceSlug, m.project.ID),
		fetchComments(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID),
		fetchAttachments(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID),
	)
}

// openCommentEditor opens the composer either blank (commentID == "", a new comment) or
// pre-filled with an existing comment's text (editing it in place).
func (m Model) openCommentEditor(commentID string) (tea.Model, tea.Cmd) {
	if m.detailItem == nil {
		return m, nil
	}
	body := ""
	if commentID != "" {
		for _, cm := range m.comments {
			if cm.ID == commentID {
				body = plainRichText(cm.CommentHTML)
				break
			}
		}
	}
	m.openEditor("comment", "Write a comment...", body, 5)
	m.editingCommentID = commentID
	return m, nil
}

// openDescriptionEditor opens the work item's description for editing, seeded with the
// current description as plain text — the same block structure the detail screen renders,
// which plainToHTML turns back into Plane's paragraph-per-line editor HTML on save.
func (m Model) openDescriptionEditor() (tea.Model, tea.Cmd) {
	if m.detailItem == nil {
		return m, nil
	}
	m.openEditor("description", "Describe this work item...", plainRichText(m.detailItem.DescriptionHTML), 12)
	return m, nil
}

// openEditor puts the shared textarea into the given mode, sized to the terminal.
func (m *Model) openEditor(mode, placeholder, body string, height int) {
	m.setError(nil)
	m.editorMode = mode
	m.editingCommentID = ""
	m.editor.Reset()
	m.editor.Placeholder = placeholder
	if m.width > 4 {
		w := m.width - 4
		if w > 70 {
			w = 70
		}
		m.editor.SetWidth(w)
	}
	if m.height <= 0 || m.height > height+12 {
		m.editor.SetHeight(height)
	} else {
		m.editor.SetHeight(minEditorHeight)
	}
	m.editor.SetValue(body)
	m.editorOn = true
	m.editor.Focus()
}

// minEditorHeight is the smallest the editor is squeezed to on a short terminal.
const minEditorHeight = 4

// closeEditor puts the editor away without saving anything. It leaves any new-item fields
// (creatingItem, newItemStateID, ...) alone — callers that are actually abandoning or
// finishing a creation call resetNewItem themselves; the title editor's own ctrl+s (below)
// needs newItemStateID to survive past this call, into the review step that follows it.
func (m *Model) closeEditor() {
	m.editorOn = false
	m.editorMode = ""
	m.editingCommentID = ""
	m.editor.Blur()
	m.editor.Reset()
}

// updateEditor drives the shared textarea for a comment, a description, and a new work item's
// title — one consolidated scheme across all three: plain enter saves/submits, alt+enter (or
// ctrl+j) inserts a newline instead. bubbletea (no Kitty keyboard protocol support) cannot tell
// a real ctrl+enter or shift+enter from plain enter on a standard terminal — both arrive as the
// same byte — so alt+enter (reliably reported, via ESC+CR) and ctrl+j (a real, distinct
// linefeed byte) are the two working stand-ins.
func (m Model) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			wasNewItem := m.editorMode == "new-item"
			m.closeEditor()
			if wasNewItem {
				m.resetNewItem()
			}
			return m, nil
		case "alt+enter", "ctrl+j":
			m.editor.InsertRune('\n')
			return m, nil
		case "enter":
			if m.editorMode == "new-item" {
				name := strings.TrimSpace(m.editor.Value())
				if name == "" {
					return m, nil
				}
				m.newItemName = name
				m.closeEditor()
				return m, nil
			}
			if m.detailItem == nil {
				return m, nil
			}
			if m.editorMode == "description" {
				// Clearing a description is a legitimate edit, unlike posting an empty
				// comment, so this one is saved exactly as typed.
				html := plainToHTML(m.editor.Value())
				m.status = "Saving description..."
				return m, updateWorkItem(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID,
					map[string]any{"description_html": html})
			}
			if strings.TrimSpace(m.editor.Value()) == "" {
				return m, nil
			}
			html := plainToHTML(m.editor.Value())
			m.status = "Saving comment..."
			if m.editingCommentID != "" {
				return m, editComment(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID, m.editingCommentID, html)
			}
			return m, createComment(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID, html)
		}
	}
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	return m, cmd
}

// handleWorkItemLoaded applies the background re-fetch of the work item the detail screen
// has open (see the board's "enter" key): the screen renders the board's cached copy right
// away and this swaps in the server's copy when it lands, with no loading screen in between.
func (m Model) handleWorkItemLoaded(msg workItemLoadedMsg) (tea.Model, tea.Cmd) {
	m.detailLoading = false
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	for i := range m.items {
		if m.items[i].ID == msg.item.ID {
			m.items[i] = *msg.item
			break
		}
	}
	// Only adopt it if the user is still looking at the same work item: by the time a slow
	// request lands they may have gone back and opened a different card.
	if m.detailItem != nil && m.detailItem.ID == msg.item.ID {
		m.detailItem = msg.item
	}
	return m, nil
}

func (m Model) handleComments(msg commentsMsg) (tea.Model, tea.Cmd) {
	// The 30s auto-refresh (handleBoardTick) re-fetches comments the same way opening the
	// detail screen does, so this also runs every 30s while it is open. wasInitialLoad tells
	// the two apart: openWorkItem clears m.comments to nil before firing the fetch, a
	// background refresh does not.
	wasInitialLoad := m.comments == nil
	m.commentsLoading = false
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	if !wasInitialLoad && commentsEqual(m.comments, msg.items) {
		// Nothing changed (no new/edited/deleted comment): leave the thread, the cursor and
		// the scroll position exactly as they are rather than replacing them wholesale every
		// 30s, which is what made the pane blink.
		return m, nil
	}
	focusID := ""
	if !wasInitialLoad && m.commentCursor >= 0 && m.commentCursor < len(m.comments) {
		focusID = m.comments[m.commentCursor].ID
	}
	m.comments = msg.items
	// Comments render oldest first, so the most recently active discussion is the last one —
	// open the detail screen focused there rather than on the oldest comment. A background
	// refresh instead keeps pointing at the same comment (by ID) the user was already on, when
	// it is still there.
	m.commentCursor = len(m.comments) - 1
	if focusID != "" {
		for i, cm := range m.comments {
			if cm.ID == focusID {
				m.commentCursor = i
				break
			}
		}
	}
	m.scrollCommentsToCursor()
	return m, nil
}

// commentsEqual reports whether two comment threads are identical — same comments, in the same
// order, with the same text and edited-at timestamp. api.Comment is a plain string-only struct,
// so its values compare with ==.
func commentsEqual(a, b []api.Comment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (m Model) handleCommentSaved(msg commentSavedMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	editingID := m.editingCommentID
	m.closeEditor()
	_ = editingID
	if msg.editing {
		for i := range m.comments {
			if m.comments[i].ID == msg.comment.ID {
				m.comments[i] = *msg.comment
				break
			}
		}
	} else {
		m.comments = append(m.comments, *msg.comment)
		m.commentCursor = len(m.comments) - 1
	}
	m.scrollCommentsToCursor()
	return m, nil
}

// memberName resolves a user ID to the person's full name ("Jane Doe"), falling back to
// their Plane handle and then their email. The detail screen has room for real names, so it
// uses these rather than the compact handles the filter pickers show.
func (m Model) memberName(id string) string {
	for _, mem := range m.members {
		if mem.ID == id {
			return mem.FullName()
		}
	}
	if m.user != nil && m.user.ID == id {
		return fmtUser(m.user)
	}
	return "unknown"
}

func formatTimestamp(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return s
	}
	return t.Local().Format("2006-01-02 15:04")
}

// detailHints are the detail screen's key hints, kept as separate chunks so packHints can
// wrap them at a word boundary rather than letting the terminal split one mid-hint.
var detailHints = [][2]string{
	{"s", "state"}, {"y", "priority"}, {"A", "assignee"}, {"T", "labels"}, {"d", "description"}, {"g", "parent"},
	{"S", "sub-tasks"}, {"u", "copy url"}, {"tab", "switch pane"}, {"j/k", "scroll"}, {"n/p", "next/prev comment"},
	{"o", "open link"}, {"f", "attachments"}, {"c", "add comment"}, {"e", "edit comment"}, {"r", "refresh"},
	{"esc", "back"}, {"q", "quit"},
}

func (m Model) viewDetail() string {
	item := m.detailItem
	if item == nil {
		return "No work item selected.\n\n" + m.footer(helpStyle.Render("esc  back"))
	}
	_, height := m.termSize()
	width, descRows, commentsRows := m.detailLayout()

	header, meta := m.detailHeader(), m.detailMeta(width)

	descHeader, commentsHeader := "Description", fmt.Sprintf("Comments (%d)", len(m.comments))
	if m.detailFocus == detailPaneDescription {
		descHeader, commentsHeader = columnHeaderFocusedStyle.Render(descHeader), columnHeaderStyle.Render(commentsHeader)
	} else {
		descHeader, commentsHeader = columnHeaderStyle.Render(descHeader), columnHeaderFocusedStyle.Render(commentsHeader)
	}

	descVP := m.descViewport
	descVP.Width, descVP.Height = width, descRows
	descVP.SetContent(m.descPaneContent(width))

	commentsContent, _, _ := m.commentsContent(width)
	commentsVP := m.commentsViewport
	commentsVP.Width, commentsVP.Height = width, commentsRows
	commentsVP.SetContent(commentsContent)

	out := header + "\n\n" + meta + "\n\n"
	out += descHeader + "\n" + descVP.View() + "\n\n"
	out += commentsHeader + "\n" + commentsVP.View() + "\n\n"
	out += m.detailBottom(width)

	return clipRows(out, height)
}

// detailHeader is the detail screen's title row: the work item's number and name.
func (m Model) detailHeader() string {
	return titleStyle.Render(fmt.Sprintf("#%d %s", m.detailItem.SequenceID, m.detailItem.Name))
}

// maxInlineSubTasks is how many sub-tasks the meta block lists in full before it stops and
// says how many more there are. The list is part of the screen's fixed chrome, so an item
// with thirty children would otherwise leave nothing for the description and comments panes;
// the rest are reachable through the S picker.
const maxInlineSubTasks = 5

// detailMeta is the block of fields under the title: the work item's own state, priority and
// assignees, then its place in the parent/sub-task hierarchy. Every related item is listed
// with its state and priority too — that, rather than the bare title, is what makes the list
// worth reading. Each line is clipped to width so a long related title cannot wrap onto a row
// detailLayout has not budgeted for.
func (m Model) detailMeta(width int) string {
	item := m.detailItem
	lines := []string{
		"State:     " + m.stateName(item.State),
		"Priority:  " + m.priorityLabel(item.Priority),
		"Assignees: " + m.assigneeNames(item.Assignees),
		"Labels:    " + m.labelsLine(item.Labels),
		"Parent:    " + m.parentLine(),
	}
	lines = append(lines, m.subTaskLines()...)
	lines = append(lines, m.attachmentLines()...)
	return clampLines(strings.Join(lines, "\n"), width)
}

// labelsLine is the meta block's "Labels:" value — the same label names a board card shows
// on its meta line (see labelNames in board.go), comma-joined, or an em dash for none.
func (m Model) labelsLine(ids []string) string {
	names := m.labelNames(ids, false)
	if len(names) == 0 {
		return "—"
	}
	return strings.Join(names, ", ")
}

// parentLine is the meta block's "Parent:" value.
func (m Model) parentLine() string {
	item := m.detailItem
	if item.Parent == "" {
		return "—"
	}
	parent := m.parentOf(*item)
	if parent == nil {
		// Either the on-demand fetch has not landed yet or it failed — most often because
		// the parent is archived, which hides it from every list this client can read.
		return helpStyle.Render("(not available)")
	}
	return m.relationSummary(*parent) + "  " + helpStyle.Render("g to open")
}

// subTaskLines are the meta block's "Sub-tasks:" row and the indented list under it.
//
// The list is dropped down to its count row while a picker or an editor is open: those are
// drawn below the two panes and are the tallest thing on the screen, and on a short terminal
// their own bottom rows (a picker's "esc cancel", an editor's save hint) are what would be
// clipped to make room for a list of sub-tasks the picker is already showing.
func (m Model) subTaskLines() []string {
	subs := m.subIssues(m.detailItem.ID)
	if len(subs) == 0 {
		return []string{"Sub-tasks: —"}
	}
	out := []string{fmt.Sprintf("Sub-tasks: %d  %s", len(subs), helpStyle.Render("S to jump"))}
	if m.pickerOpen != "" || m.editorOn {
		return out
	}
	for i, sub := range subs {
		if i >= maxInlineSubTasks {
			out = append(out, helpStyle.Render(fmt.Sprintf("           … %d more", len(subs)-maxInlineSubTasks)))
			break
		}
		out = append(out, "           "+m.relationSummary(sub))
	}
	return out
}

// attachmentLines are the meta block's "Attachments:" row and the indented list under it —
// subTaskLines' equivalent for attachments, preloaded whenever the item is opened or refreshed
// (openWorkItem, refreshDetail) rather than only when the "f" picker opens, so the count/list
// show up here without the user needing to open that picker first.
func (m Model) attachmentLines() []string {
	if m.attachmentsLoading {
		return []string{"Attachments: " + helpStyle.Render("loading...")}
	}
	if len(m.attachments) == 0 {
		return []string{"Attachments: —"}
	}
	out := []string{fmt.Sprintf("Attachments: %d  %s", len(m.attachments), helpStyle.Render("f to manage"))}
	if m.pickerOpen != "" || m.editorOn {
		return out
	}
	for i, att := range m.attachments {
		if i >= maxInlineSubTasks {
			out = append(out, helpStyle.Render(fmt.Sprintf("             … %d more", len(m.attachments)-maxInlineSubTasks)))
			break
		}
		out = append(out, "             "+att.Name()+"  "+helpStyle.Render(humanSize(att.Size())))
	}
	return out
}

// detailLayout is the detail screen's row-budget math: it works out how many rows each of
// the two scrollable panes gets so that, together with the fixed chrome around them (the
// title, the meta block, each pane's own header, and the footer/editor/picker at the
// bottom), the whole screen fits m.height exactly rather than "usually" — the same discipline
// board.go's boardCardRows follows for the board's columns. Shared by viewDetail (to size
// what it renders) and the scroll helpers below (so a keypress moves the same pane height
// that will actually be drawn).
func (m Model) detailLayout() (width, descRows, commentsRows int) {
	width, height := m.termSize()
	if m.detailItem == nil {
		return width, 0, 0
	}
	header, meta := m.detailHeader(), m.detailMeta(width)
	bottom := m.detailBottom(width)

	// Everything besides the two panes: the title + blank line, the meta block + blank
	// line, each pane's own header row, a blank line between the panes and before the
	// bottom, and the footer/editor/picker itself.
	chrome := visualHeight(header, width) + 1 +
		visualHeight(meta, width) + 1 +
		1 /* Description header */ + 1 /* blank line between panes */ +
		1 /* Comments header */ + 1 /* blank line before the bottom */ +
		visualHeight(bottom, width)

	descRows, commentsRows = detailPaneRows(height, chrome, m.minPaneRows())
	return width, descRows, commentsRows
}

// overlayOpen reports whether a picker or the editor is on screen. Both are modal — the
// detail screen's own keys do nothing while one is up — so they take priority over the two
// panes when the terminal is too short for everything (see minPaneRows).
func (m Model) overlayOpen() bool {
	return m.pickerOpen != "" || m.editorOn || m.attachPathPromptOpen
}

// minPaneRows is the fewest rows each detail pane is squeezed to. With an overlay open that
// is one row apiece rather than minPaneRows: the panes are the part the user is not looking
// at, and holding them at three rows each is what used to push a tall picker's last options
// (and its "esc cancel" row) off the bottom of an 80x24 terminal.
func (m Model) minPaneRows() int {
	if m.overlayOpen() {
		return 1
	}
	return minPaneRows
}

// detailPaneRows splits what detailLayout has left after its chrome between the description
// and comments panes. The two always sum to exactly height-chrome once that is at least
// 2*floor, which is what makes the panes plus the chrome fit m.height exactly; below that
// floor the final clipRows in viewDetail is what keeps the frame from overflowing, the same
// fallback boardCardRows leaves to board.go's own clipRows.
func detailPaneRows(height, chrome, floor int) (descRows, commentsRows int) {
	avail := height - chrome
	if avail < floor*2 {
		avail = floor * 2
	}
	descRows = avail / 2
	commentsRows = avail - descRows
	return descRows, commentsRows
}

// detailBottom is everything drawn below the two panes: the comment/description editor in
// place of the footer while one is open, the ordinary hint/error/status footer otherwise,
// and the picker overlay after either when a picker is open.
func (m Model) detailBottom(width int) string {
	var bottom string
	if m.editorOn {
		title := "New comment"
		switch {
		case m.editorMode == "description":
			title = "Edit description"
		case m.editingCommentID != "":
			title = "Edit comment"
		}
		hint := "enter  save    alt+enter/ctrl+j  new line    esc  cancel"
		bottom = focusedInputStyle.Render(columnHeaderStyle.Render(title) + "\n" + m.editor.View() + "\n" +
			helpStyle.Render(hint))
	} else {
		// A picker (or the attach-file prompt) is modal: every key in detailHints is inactive
		// while one is open, and it carries its own hint row, so the screen's hints go — which
		// on a short terminal is also what buys it the rows it needs to be drawn whole.
		hint := ""
		if m.pickerOpen == "" && !m.attachPathPromptOpen {
			hint = packHints(detailHints, width)
		}
		if loader := m.detailLoader(); loader != "" {
			if hint != "" {
				hint += "\n"
			}
			hint += helpStyle.Render(loader)
		}
		bottom = m.footer(hint)
	}
	if m.pickerOpen != "" {
		bottom += "\n\n" + m.viewPicker()
	}
	if m.attachPathPromptOpen {
		bottom += "\n\n" + m.viewAttachPathPrompt()
	}
	return bottom
}

// descPaneContent is the work item's description, pre-wrapped to width. The viewport only
// scrolls whole "\n"-separated lines, not the terminal rows a long line wraps onto, so it is
// wrapped here rather than left to the viewport's own rendering — otherwise a single long
// line could occupy more on-screen rows than the scroll math accounts for.
func (m Model) descPaneContent(width int) string {
	desc := formatRichText(m.detailItem.DescriptionHTML)
	if desc == "" {
		return helpStyle.Render("(no description)")
	}
	return ansi.Wrap(desc, width, "")
}

// commentsContent renders the comment thread's body — everything but the "Comments (N)"
// header, which stays outside the scrollable pane so it never scrolls out of view — wrapped
// to width for the same reason descPaneContent is. It also reports the line range the
// focused comment ends up on, so scrollCommentsToCursor can scroll it into view.
func (m Model) commentsContent(width int) (content string, focusStart, focusEnd int) {
	// Only the initial fetch (no comments yet) shows the loading placeholder. The 30s
	// auto-refresh also sets commentsLoading while it re-fetches, but the thread already on
	// screen must stay put until handleComments actually has something different to show —
	// otherwise this blanked the pane on every tick, blinking it even when nothing changed.
	if m.commentsLoading && len(m.comments) == 0 {
		return helpStyle.Render("Loading comments..."), 0, 0
	}
	if len(m.comments) == 0 {
		return helpStyle.Render("No comments yet."), 0, 0
	}
	var b strings.Builder
	line := 0
	writeLine := func(s string) {
		wrapped := ansi.Wrap(s, width, "")
		b.WriteString(wrapped)
		b.WriteString("\n")
		line += strings.Count(wrapped, "\n") + 1
	}
	for i, cm := range m.comments {
		start := line
		header := fmt.Sprintf("%s  %s", m.memberName(cm.Actor), formatTimestamp(cm.CreatedAt))
		if cm.EditedAt != "" {
			header += "  (edited)"
		}
		style := helpStyle
		if i == m.commentCursor {
			header = "> " + header
			style = commentHeaderFocusedStyle
		} else {
			header = "  " + header
		}
		writeLine(style.Render(header))
		body := formatRichText(cm.CommentHTML)
		for _, l := range strings.Split(body, "\n") {
			writeLine("    " + l)
		}
		if i == m.commentCursor {
			focusStart, focusEnd = start, line-1
		}
	}
	return strings.TrimSuffix(b.String(), "\n"), focusStart, focusEnd
}

// ensureVisible scrolls vp by the minimum amount so the [start, end] line range is on
// screen, rather than resetting its scroll position outright.
func ensureVisible(vp *viewport.Model, start, end int) {
	if start < vp.YOffset {
		vp.SetYOffset(start)
		return
	}
	if end > vp.YOffset+vp.Height-1 {
		vp.SetYOffset(end - vp.Height + 1)
	}
}

// scrollCommentsToCursor keeps the focused comment visible in the comments pane — called
// whenever commentCursor moves and when the thread first loads (or a comment is posted or
// edited), so the detail screen opens with the latest comment in view rather than merely
// cursor-selected off-screen.
func (m *Model) scrollCommentsToCursor() {
	if m.detailItem == nil {
		return
	}
	width, _, commentsRows := m.detailLayout()
	content, focusStart, focusEnd := m.commentsContent(width)
	m.commentsViewport.Width, m.commentsViewport.Height = width, commentsRows
	m.commentsViewport.SetContent(content)
	ensureVisible(&m.commentsViewport, focusStart, focusEnd)
}

// scrollDescription moves the description pane's own scroll position by n lines — j/k
// scroll it while it has focus, instead of moving the comment cursor.
func (m *Model) scrollDescription(n int) {
	if m.detailItem == nil {
		return
	}
	width, descRows, _ := m.detailLayout()
	m.descViewport.Width, m.descViewport.Height = width, descRows
	m.descViewport.SetContent(m.descPaneContent(width))
	if n > 0 {
		m.descViewport.ScrollDown(n)
	} else {
		m.descViewport.ScrollUp(-n)
	}
}

// scrollComments moves the comments pane's own scroll position by n lines, independently of
// which comment is focused — j/k while the comments pane has focus, so a comment longer than
// the pane can be read a line at a time. n/p (see updateDetail) are what move commentCursor and
// jump straight to the next/previous comment instead.
func (m *Model) scrollComments(n int) {
	if m.detailItem == nil {
		return
	}
	width, _, commentsRows := m.detailLayout()
	content, _, _ := m.commentsContent(width)
	m.commentsViewport.Width, m.commentsViewport.Height = width, commentsRows
	m.commentsViewport.SetContent(content)
	if n > 0 {
		m.commentsViewport.ScrollDown(n)
	} else {
		m.commentsViewport.ScrollUp(-n)
	}
}

// resetDetailView clears the previous work item's scroll positions and pane focus — called
// whenever the board opens a (possibly different) card, so a freshly opened item never
// starts out scrolled to wherever the last one was left.
func (m *Model) resetDetailView() {
	m.detailFocus = detailPaneComments
	m.descViewport = viewport.Model{}
	m.commentsViewport = viewport.Model{}
}

// assigneeNames renders a work item's assignees as a comma-separated list of full names.
func (m Model) assigneeNames(ids []string) string {
	if len(ids) == 0 {
		return "—"
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		names = append(names, m.memberName(id))
	}
	return strings.Join(names, ", ")
}

// detailLoader is the footer's background-activity line: opening a card shows its cached
// copy immediately and refreshes it (and its comments) behind the scenes, and this is what
// says so.
func (m Model) detailLoader() string {
	var what []string
	if m.detailLoading {
		what = append(what, "work item")
	}
	if m.commentsLoading {
		what = append(what, "comments")
	}
	if len(what) == 0 {
		return ""
	}
	return "⟳ refreshing " + strings.Join(what, " and ") + "…"
}
