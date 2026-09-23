package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent = lipgloss.Color("39")
	colorMuted  = lipgloss.Color("240")
	colorError  = lipgloss.Color("203")
	colorGood   = lipgloss.Color("42")

	// colorSelect marks the item under the cursor (a card, or a line in any of the "> "
	// picker lists): a dark grey rather than colorAccent's blue, which used to double as both
	// the selection color and the branding color everywhere else (titles, focused borders).
	colorSelect = lipgloss.Color("238")

	// colorCommentFocused is the comment header (author + datetime) color when that comment
	// is under the cursor — a step lighter than helpStyle's colorMuted so the focused comment
	// reads as slightly brighter than the rest, without being as loud as colorSelect.
	colorCommentFocused = lipgloss.Color("250")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)

	helpStyle = lipgloss.NewStyle().Foreground(colorMuted)

	commentHeaderFocusedStyle = lipgloss.NewStyle().Foreground(colorCommentFocused)

	errorStyle = lipgloss.NewStyle().Foreground(colorError)

	focusedInputStyle = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorAccent).Padding(0, 1)

	// columnStyle has no border: a board's state columns used to be boxed in a rounded
	// border, which cost every column 2 terminal columns (colFrame) it could otherwise give
	// to card titles. The focused column is now marked on its header (columnHeaderFocusedStyle)
	// instead of a colored border.
	columnStyle = lipgloss.NewStyle().Padding(0, 1)

	columnHeaderStyle = lipgloss.NewStyle().Bold(true)

	columnHeaderFocusedStyle = columnHeaderStyle.Foreground(colorSelect).Underline(true)

	helpSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(colorGood)

	helpKeyStyle = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	cardStyle = lipgloss.NewStyle().Padding(0, 1)

	cardSelectedStyle = cardStyle.Background(colorSelect).Foreground(lipgloss.Color("255"))

	priorityStyles = map[string]lipgloss.Style{
		"urgent": lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
		"high":   lipgloss.NewStyle().Foreground(lipgloss.Color("208")),
		"medium": lipgloss.NewStyle().Foreground(lipgloss.Color("220")),
		"low":    lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		"none":   lipgloss.NewStyle().Foreground(colorMuted),
	}
)

func priorityLabel(p string) string {
	if p == "" {
		p = "none"
	}
	style, ok := priorityStyles[p]
	if !ok {
		style = priorityStyles["none"]
	}
	return style.Render(p)
}
