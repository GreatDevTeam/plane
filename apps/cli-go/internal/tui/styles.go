package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorAccent = lipgloss.Color("39")
	colorMuted  = lipgloss.Color("240")
	colorError  = lipgloss.Color("203")
	colorGood   = lipgloss.Color("42")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)

	helpStyle = lipgloss.NewStyle().Foreground(colorMuted)

	errorStyle = lipgloss.NewStyle().Foreground(colorError)

	focusedInputStyle = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorAccent).Padding(0, 1)

	columnStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1)

	columnFocusedStyle = columnStyle.BorderForeground(colorAccent)

	columnHeaderStyle = lipgloss.NewStyle().Bold(true)

	cardStyle = lipgloss.NewStyle().Padding(0, 1)

	cardSelectedStyle = cardStyle.Background(colorAccent).Foreground(lipgloss.Color("0"))

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
