package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case "c":
		return m.openCommentEditor("")
	case "d":
		return m.openDescriptionEditor()
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
	case "up", "k":
		if m.commentCursor > 0 {
			m.commentCursor--
		}
	case "down", "j":
		if m.commentCursor < len(m.comments)-1 {
			m.commentCursor++
		}
	}
	return m, nil
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

// closeEditor puts the editor away without saving anything.
func (m *Model) closeEditor() {
	m.editorOn = false
	m.editorMode = ""
	m.editingCommentID = ""
	m.editor.Blur()
	m.editor.Reset()
}

// updateEditor drives the shared textarea for both a comment and a description.
func (m Model) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.closeEditor()
			return m, nil
		case "ctrl+s":
			if m.detailItem == nil {
				return m, nil
			}
			html := plainToHTML(m.editor.Value())
			if m.editorMode == "description" {
				// Clearing a description is a legitimate edit, unlike posting an empty
				// comment, so this one is saved exactly as typed.
				m.status = "Saving description..."
				return m, updateWorkItem(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID,
					map[string]any{"description_html": html})
			}
			if strings.TrimSpace(m.editor.Value()) == "" {
				return m, nil
			}
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
	m.commentsLoading = false
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	m.comments = msg.items
	// Comments render oldest first, so the most recently active discussion is the last one —
	// open the detail screen focused there rather than on the oldest comment.
	m.commentCursor = len(m.comments) - 1
	return m, nil
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
var detailHints = []string{
	"s  state", "y  priority", "d  description", "j/k  comment", "c  add comment",
	"e  edit comment", "esc  back", "q  quit",
}

func (m Model) viewDetail() string {
	item := m.detailItem
	if item == nil {
		return "No work item selected.\n\n" + m.footer("esc  back")
	}
	state := "unknown"
	if st := m.findState(item.State); st != nil {
		state = st.Name
	}

	out := titleStyle.Render(fmt.Sprintf("#%d %s", item.SequenceID, item.Name)) + "\n\n"
	out += fmt.Sprintf("State:     %s\n", state)
	out += fmt.Sprintf("Priority:  %s\n", priorityLabel(item.Priority))
	out += fmt.Sprintf("Assignees: %s\n", m.assigneeNames(item.Assignees))
	out += "\n"

	desc := formatRichText(item.DescriptionHTML)
	if desc == "" {
		desc = helpStyle.Render("(no description)")
	}
	out += "Description:\n" + desc + "\n\n"

	out += m.viewComments()

	if m.editorOn {
		title := "New comment"
		switch {
		case m.editorMode == "description":
			title = "Edit description"
		case m.editingCommentID != "":
			title = "Edit comment"
		}
		out += "\n" + focusedInputStyle.Render(columnHeaderStyle.Render(title)+"\n"+m.editor.View()+"\n"+
			helpStyle.Render("ctrl+s  save    esc  cancel"))
	} else {
		width, _ := m.termSize()
		hint := packHints(detailHints, width)
		if loader := m.detailLoader(); loader != "" {
			hint += "\n" + loader
		}
		out += m.footer(hint)
	}
	if m.pickerOpen != "" {
		out += "\n\n" + m.viewPicker()
	}
	return out
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

func (m Model) viewComments() string {
	out := columnHeaderStyle.Render(fmt.Sprintf("Comments (%d)", len(m.comments))) + "\n"
	if m.commentsLoading {
		return out + helpStyle.Render("Loading comments...") + "\n"
	}
	if len(m.comments) == 0 {
		return out + helpStyle.Render("No comments yet.") + "\n"
	}
	for i, cm := range m.comments {
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
		out += style.Render(header) + "\n"
		body := formatRichText(cm.CommentHTML)
		for _, line := range strings.Split(body, "\n") {
			out += "    " + line + "\n"
		}
	}
	return out
}
