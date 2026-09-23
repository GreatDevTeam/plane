package tui

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.pickerOpen != "" {
		return m.updatePicker(msg)
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
	}
	return m, nil
}

var htmlTagRE = regexp.MustCompile(`<[^>]*>`)

func stripHTML(html string) string {
	text := htmlTagRE.ReplaceAllString(html, "")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	return strings.TrimSpace(text)
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

	desc := stripHTML(item.DescriptionHTML)
	if desc == "" {
		desc = helpStyle.Render("(no description)")
	}
	out += "Description:\n" + desc + "\n\n"

	out += m.footer("s  change state    y  change priority    esc/backspace  back    q  quit")
	if m.pickerOpen != "" {
		out += "\n\n" + m.viewPicker()
	}
	return out
}
