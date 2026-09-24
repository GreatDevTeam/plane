package tui

import (
	"fmt"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
)

// openFocusedLinks is the detail screen's "o" key: it collects the links out of whichever pane
// has focus (the description, or the focused comment) and opens the one link directly, or —
// when there is more than one — opens a picker so the user chooses which.
func (m Model) openFocusedLinks() (tea.Model, tea.Cmd) {
	if m.detailItem == nil {
		return m, nil
	}
	var html string
	switch {
	case m.detailFocus == detailPaneDescription:
		html = m.detailItem.DescriptionHTML
	case m.commentCursor >= 0 && m.commentCursor < len(m.comments):
		html = m.comments[m.commentCursor].CommentHTML
	}
	links := extractLinks(html)
	switch len(links) {
	case 0:
		m.setError(fmt.Errorf("no links found"))
		return m, nil
	case 1:
		return m.openLinkInBrowser(links[0])
	default:
		m.setError(nil)
		m.linkOptions = links
		m.pickerOpen = "links"
		m.pickerIdx = 0
		return m, nil
	}
}

// openURLFunc is what openLinkInBrowser actually calls — a package variable, rather than
// calling openURL directly, so tests can swap it out instead of really spawning a browser.
var openURLFunc = openURL

// openLinkInBrowser hands url to the OS's default handler.
func (m Model) openLinkInBrowser(url string) (tea.Model, tea.Cmd) {
	if err := openURLFunc(url); err != nil {
		m.setError(fmt.Errorf("open link: %w", err))
		return m, nil
	}
	m.setError(nil)
	m.status = "Opened " + url
	return m, nil
}

// openURL launches the OS's default handler for url — the terminal plane-cli runs in has no
// browser of its own to render one in.
func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
