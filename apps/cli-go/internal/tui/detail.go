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
	if m.commentEditorOn {
		return m.updateCommentEditor(msg)
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
	m.setError(nil)
	m.editingCommentID = commentID
	m.commentEditor.Reset()
	if m.width > 4 {
		w := m.width - 4
		if w > 70 {
			w = 70
		}
		m.commentEditor.SetWidth(w)
	}
	if commentID != "" {
		for _, cm := range m.comments {
			if cm.ID == commentID {
				m.commentEditor.SetValue(plainRichText(cm.CommentHTML))
				break
			}
		}
	}
	m.commentEditorOn = true
	m.commentEditor.Focus()
	return m, nil
}

func (m Model) updateCommentEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.commentEditorOn = false
			m.commentEditor.Blur()
			m.commentEditor.Reset()
			return m, nil
		case "ctrl+s":
			text := strings.TrimSpace(m.commentEditor.Value())
			if text == "" || m.detailItem == nil {
				return m, nil
			}
			html := plainToHTML(m.commentEditor.Value())
			m.status = "Saving comment..."
			if m.editingCommentID != "" {
				return m, editComment(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID, m.editingCommentID, html)
			}
			return m, createComment(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID, html)
		}
	}
	var cmd tea.Cmd
	m.commentEditor, cmd = m.commentEditor.Update(msg)
	return m, cmd
}

func (m Model) handleComments(msg commentsMsg) (tea.Model, tea.Cmd) {
	m.commentsLoading = false
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	m.comments = msg.items
	if m.commentCursor >= len(m.comments) {
		m.commentCursor = len(m.comments) - 1
	}
	return m, nil
}

func (m Model) handleCommentSaved(msg commentSavedMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	m.commentEditorOn = false
	m.commentEditor.Blur()
	m.commentEditor.Reset()
	m.editingCommentID = ""
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

func (m Model) memberName(id string) string {
	for _, mem := range m.members {
		if mem.ID == id {
			return mem.Name()
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
	out += fmt.Sprintf("Assignees: %d\n", len(item.Assignees))
	out += "\n"

	desc := formatRichText(item.DescriptionHTML)
	if desc == "" {
		desc = helpStyle.Render("(no description)")
	}
	out += "Description:\n" + desc + "\n\n"

	out += m.viewComments()

	if m.commentEditorOn {
		title := "New comment"
		if m.editingCommentID != "" {
			title = "Edit comment"
		}
		out += "\n" + focusedInputStyle.Render(columnHeaderStyle.Render(title)+"\n"+m.commentEditor.View()+"\n"+
			helpStyle.Render("ctrl+s  save    esc  cancel"))
	} else {
		out += m.footer("s  state    y  priority    j/k  select comment    c  comment    e  edit comment    esc/backspace  back    q  quit")
	}
	if m.pickerOpen != "" {
		out += "\n\n" + m.viewPicker()
	}
	return out
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
		if i == m.commentCursor {
			header = "> " + header
		} else {
			header = "  " + header
		}
		out += helpStyle.Render(header) + "\n"
		body := formatRichText(cm.CommentHTML)
		for _, line := range strings.Split(body, "\n") {
			out += "    " + line + "\n"
		}
	}
	return out
}
