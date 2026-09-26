// Command plane-cli is a terminal client for Plane's work items: connect to a server with an
// API token or email/password, browse projects as kanban boards grouped by state, and open
// or change work items — all from the keyboard. Built for Ubuntu.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/config"
	"github.com/makeplane/plane/apps/cli-go/internal/logging"
	"github.com/makeplane/plane/apps/cli-go/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "plane-cli: could not read config: %v\n", err)
		os.Exit(1)
	}

	// Logging is diagnostic only: a failure to open it (e.g. an unwritable config dir) must not
	// stop plane-cli from running, just leave it without a local warning/error trail.
	if err := logging.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "plane-cli: could not open log file: %v\n", err)
	}
	defer logging.Close()

	p := tea.NewProgram(tui.New(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "plane-cli: %v\n", err)
		os.Exit(1)
	}
}
