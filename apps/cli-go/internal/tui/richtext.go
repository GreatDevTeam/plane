package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	reBold     = regexp.MustCompile(`(?is)<(strong|b)[^>]*>(.*?)</(strong|b)>`)
	reItalic   = regexp.MustCompile(`(?is)<(em|i)[^>]*>(.*?)</(em|i)>`)
	reCode     = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code>`)
	reListItem = regexp.MustCompile(`(?i)<li[^>]*>`)
	reHref     = regexp.MustCompile(`(?i)href="([^"]+)"`)
	rePlainURL = regexp.MustCompile(`https?://[^\s<>"']+`)
	// </li> is deliberately excluded: <li> already opens the item on its own line via
	// reListItem, so also emitting a newline on close would leave a blank line between items.
	reBlockClose = regexp.MustCompile(`(?i)</(p|h1|h2|h3|h4|h5|h6|div|blockquote|ul|ol)>`)
	reBreak      = regexp.MustCompile(`(?i)<br\s*/?>`)
	reAnyTag     = regexp.MustCompile(`<[^>]*>`)
	reBlankLines = regexp.MustCompile(`\n{3,}`)

	richBoldStyle = lipgloss.NewStyle().Bold(true)
	richItalic    = lipgloss.NewStyle().Italic(true)
	richCode      = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

// formatRichText renders Plane's editor HTML as readable terminal text: paragraphs and
// list items become their own lines (with a leading "- " for list items), and bold/italic/
// code spans keep their emphasis via ANSI styling.
func formatRichText(html string) string {
	return richText(html, true)
}

// plainRichText is the same block-structure conversion without ANSI styling. Used to seed
// the plain-text comment editor, where embedded escape codes would corrupt what gets typed.
func plainRichText(html string) string {
	return richText(html, false)
}

func richText(html string, styled bool) string {
	s := html
	if styled {
		s = reBold.ReplaceAllStringFunc(s, func(m string) string {
			return richBoldStyle.Render(reAnyTag.ReplaceAllString(m, ""))
		})
		s = reItalic.ReplaceAllStringFunc(s, func(m string) string {
			return richItalic.Render(reAnyTag.ReplaceAllString(m, ""))
		})
		s = reCode.ReplaceAllStringFunc(s, func(m string) string {
			return richCode.Render(reAnyTag.ReplaceAllString(m, ""))
		})
	}
	s = reListItem.ReplaceAllString(s, "\n- ")
	s = reBreak.ReplaceAllString(s, "\n")
	s = reBlockClose.ReplaceAllString(s, "\n")
	s = reAnyTag.ReplaceAllString(s, "")
	s = unescapeHTML(s)

	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	s = strings.Join(lines, "\n")
	s = reBlankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func unescapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&rarr;", "→")
	return s
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// extractLinks pulls every URL out of a Plane editor HTML fragment: both an <a href="...">'s
// target and any bare "http(s)://..." typed as plain text, in the order they first appear, with
// duplicates dropped. Used by the detail screen's "open link" action (see openFocusedLinks).
func extractLinks(html string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(u string) {
		u = strings.TrimRight(u, ".,;:)]}>\"'")
		if u == "" || seen[u] {
			return
		}
		seen[u] = true
		out = append(out, u)
	}
	for _, m := range reHref.FindAllStringSubmatch(html, -1) {
		add(unescapeHTML(m[1]))
	}
	for _, m := range rePlainURL.FindAllString(html, -1) {
		add(m)
	}
	return out
}

// plainToHTML turns the plain text out of the comment editor into the paragraph-per-line
// HTML Plane's editor produces, so a comment written here still renders correctly there.
func plainToHTML(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	var b strings.Builder
	for _, l := range lines {
		b.WriteString("<p>")
		b.WriteString(escapeHTML(l))
		b.WriteString("</p>")
	}
	if b.Len() == 0 {
		return "<p></p>"
	}
	return b.String()
}
