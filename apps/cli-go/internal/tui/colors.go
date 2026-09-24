package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
	"github.com/makeplane/plane/apps/cli-go/internal/config"
)

// openColorTargetPicker opens the board's local color-settings screen ("C"): a picker over
// every state in this project, then every label, so the user can give any of them their own
// display color without touching what is actually stored in Plane.
func (m *Model) openColorTargetPicker() {
	m.pickerOpen = "color-target"
	m.pickerIdx = 0
}

// colorTargetAt resolves a color-target picker row back to the state or label it stands for.
// Every state comes first, then every label — the same order pickerOptionCount's "color-target"
// case sums their lengths in, and viewPicker renders them in.
func (m Model) colorTargetAt(idx int) (st *api.State, l *api.Label, ok bool) {
	if idx < 0 {
		return nil, nil, false
	}
	if idx < len(m.states) {
		s := m.states[idx]
		return &s, nil, true
	}
	idx -= len(m.states)
	if idx < len(m.labels) {
		lb := m.labels[idx]
		return nil, &lb, true
	}
	return nil, nil, false
}

// openColorPrompt opens the hex-entry box for one state's or label's local color override,
// pre-filled with its current effective color (the override already set for it, if any,
// otherwise its color from Plane — see effectiveStateColor/effectiveLabelColor) so re-opening
// it to tweak a color does not start from blank.
func (m *Model) openColorPrompt(kind, id, currentHex string) {
	m.setError(nil)
	m.colorTargetKind = kind
	m.colorTargetID = id
	m.colorInput.Reset()
	m.colorInput.SetValue(strings.TrimPrefix(currentHex, "#"))
	m.colorInput.CursorEnd()
	m.colorInput.Focus()
	m.colorPromptOpen = true
}

// closeColorPrompt puts the hex-entry box away without saving anything.
func (m Model) closeColorPrompt() Model {
	m.colorPromptOpen = false
	m.colorInput.Blur()
	m.colorInput.Reset()
	return m
}

// updateColorPrompt drives the hex-entry box: enter validates the typed hex and saves it as a
// local override (config.SetStateColor/SetLabelColor), persisted to disk the same way
// toggleHiddenColumn's column collapse is.
func (m Model) updateColorPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m.closeColorPrompt(), nil
		case "enter":
			hex := "#" + strings.TrimPrefix(strings.TrimSpace(m.colorInput.Value()), "#")
			if _, _, _, ok := parseHexColor(hex); !ok {
				m.setError(fmt.Errorf("enter a 6-digit hex color, e.g. ff8800"))
				return m, nil
			}
			if m.colorTargetKind == "state" {
				m.cfg.SetStateColor(m.project.ID, m.colorTargetID, hex)
			} else {
				m.cfg.SetLabelColor(m.project.ID, m.colorTargetID, hex)
			}
			if err := config.Save(m.cfg); err != nil {
				m.setError(err)
				return m, nil
			}
			m.setError(nil)
			m.status = "Color updated"
			return m.closeColorPrompt(), nil
		}
	}
	var cmd tea.Cmd
	m.colorInput, cmd = m.colorInput.Update(msg)
	return m, cmd
}

// viewColorPrompt renders the hex-entry box.
func (m Model) viewColorPrompt() string {
	body := columnHeaderStyle.Render("Color (hex, e.g. ff8800)") + "\n" + m.colorInput.View() + "\n" +
		helpStyle.Render("enter  save    esc  cancel")
	return focusedInputStyle.Render(body)
}
