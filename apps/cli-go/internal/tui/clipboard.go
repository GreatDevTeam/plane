package tui

import (
	"fmt"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// workItemURL is a work item's URL in the web app: the same "browse" route the web app's own
// issue detail page resolves (apps/web/app/(all)/[workspaceSlug]/(projects)/browse/[workItem]/page.tsx,
// which splits the workItem route param on "-" into a project identifier and a sequence
// number).
func (m Model) workItemURL(it api.WorkItem) string {
	return fmt.Sprintf("%s/%s/browse/%s-%d", m.serverURL, m.workspaceSlug, m.project.Identifier, it.SequenceID)
}

// copyItemURL copies a work item's URL to the system clipboard — the board and detail
// screens' "u" key. A nil item (the board's u pressed with no card focused, e.g. every column
// hidden) is a no-op.
func (m Model) copyItemURL(it *api.WorkItem) (tea.Model, tea.Cmd) {
	if it == nil {
		return m, nil
	}
	url := m.workItemURL(*it)
	if err := clipboard.WriteAll(url); err != nil {
		m.setError(fmt.Errorf("copy url: %w", err))
		return m, nil
	}
	m.setError(nil)
	m.status = "Copied " + url
	return m, nil
}

// openItemInBrowser hands a work item's web-app URL to the OS's default handler — the board
// and detail screens' "U" (shift+u) key, alongside "u"'s copy-to-clipboard. A nil item (the
// board's U pressed with no card focused) is a no-op.
func (m Model) openItemInBrowser(it *api.WorkItem) (tea.Model, tea.Cmd) {
	if it == nil {
		return m, nil
	}
	return m.openLinkInBrowser(m.workItemURL(*it))
}
