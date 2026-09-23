package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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

	// colorBoardTitle is the board (project name) header's own color — white, so it stands
	// out from titleStyle's blue, which every other screen's title still uses.
	colorBoardTitle = lipgloss.Color("255")

	// colorHiddenBg/colorHiddenFg render a collapsed board column's placeholder as dimmed
	// text on a light grey background, rather than just narrow, so a hidden state visibly
	// reads as out-of-the-way at a glance.
	colorHiddenBg = lipgloss.Color("252")
	colorHiddenFg = lipgloss.Color("238")

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)

	boardTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBoardTitle)

	helpStyle = lipgloss.NewStyle().Foreground(colorMuted)

	commentHeaderFocusedStyle = lipgloss.NewStyle().Foreground(colorCommentFocused)

	errorStyle = lipgloss.NewStyle().Foreground(colorError)

	focusedInputStyle = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorAccent).Padding(0, 1)

	// columnStyle has no border: a board's state columns used to be boxed in a rounded
	// border, which cost every column 2 terminal columns (colFrame) it could otherwise give
	// to card titles. The focused column is now marked on its header instead of a colored
	// border (an underline on top of the state color for a board column — see
	// pastelStateColor in renderColumn — or columnHeaderFocusedStyle for the detail screen's
	// two panes, which have no per-state color of their own).
	columnStyle = lipgloss.NewStyle().Padding(0, 1)

	columnHeaderStyle = lipgloss.NewStyle().Bold(true)

	columnHeaderFocusedStyle = columnHeaderStyle.Foreground(colorSelect).Underline(true)

	// hiddenColumnStyle is a collapsed board column's placeholder: dimmed text on a light
	// grey background (colorHiddenBg/colorHiddenFg) rather than just columnStyle's plain
	// default colors, so a hidden state reads as visibly out-of-the-way.
	hiddenColumnStyle = columnStyle.Background(colorHiddenBg).Foreground(colorHiddenFg)

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

// pastelStateColor lightens a Plane state's own hex color (State.Color, e.g. "#16a34a") by
// blending it two-thirds of the way toward white, so a column header reads as a soft tint
// rather than the API's full-saturation swatch — that color is designed to work as a small
// dot next to a state's name in the web app, not as bold text a terminal renders for minutes
// at a stretch. Falls back to colorMuted for an empty or malformed color, e.g. a state whose
// project predates Plane assigning one.
func pastelStateColor(hex string) lipgloss.Color {
	r, g, b, ok := parseHexColor(hex)
	if !ok {
		return colorMuted
	}
	const towardWhite = 2.0 / 3.0
	pastel := func(c uint8) uint8 {
		return c + uint8((255-float64(c))*towardWhite)
	}
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", pastel(r), pastel(g), pastel(b)))
}

// parseHexColor parses a "#rrggbb" (or "rrggbb") string into its components. ok is false for
// anything else, including the empty string.
func parseHexColor(s string) (r, g, b uint8, ok bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), true
}

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
