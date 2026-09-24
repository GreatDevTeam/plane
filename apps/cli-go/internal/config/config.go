// Package config persists the last-used server connection so plane-cli does not need to
// sign in again on every run.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// Config is the on-disk connection profile plus the board preferences that should survive a
// restart (which columns are collapsed, how cards are ordered).
type Config struct {
	ServerURL     string `json:"server_url"`
	Token         string `json:"token"`
	WorkspaceSlug string `json:"workspace_slug"`

	// HiddenStates maps a project ID to the state IDs whose columns are collapsed to a
	// placeholder on that project's board.
	HiddenStates map[string][]string `json:"hidden_states,omitempty"`

	// SortMode is the board's card ordering (see tui.SortModes); "" means the order the
	// API returned the items in.
	SortMode string `json:"sort_mode,omitempty"`

	// StateColors/LabelColors override the swatch color a state/label is rendered in, per
	// project, keyed by project ID -> state/label ID -> "#rrggbb". Local display preference
	// only — it never changes the color stored on the state/label in Plane itself.
	StateColors map[string]map[string]string `json:"state_colors,omitempty"`
	LabelColors map[string]map[string]string `json:"label_colors,omitempty"`
}

// StateColorFor returns the local override color for a state, or "" when none is set.
func (c Config) StateColorFor(projectID, stateID string) string {
	return c.StateColors[projectID][stateID]
}

// LabelColorFor returns the local override color for a label, or "" when none is set.
func (c Config) LabelColorFor(projectID, labelID string) string {
	return c.LabelColors[projectID][labelID]
}

// SetStateColor records a local override color for a state (hex, e.g. "#ff8800").
func (c *Config) SetStateColor(projectID, stateID, hex string) {
	setNestedColor(&c.StateColors, projectID, stateID, hex)
}

// SetLabelColor records a local override color for a label (hex, e.g. "#ff8800").
func (c *Config) SetLabelColor(projectID, labelID, hex string) {
	setNestedColor(&c.LabelColors, projectID, labelID, hex)
}

// setNestedColor sets m[projectID][id] = hex, allocating either map as needed.
func setNestedColor(m *map[string]map[string]string, projectID, id, hex string) {
	if *m == nil {
		*m = make(map[string]map[string]string)
	}
	if (*m)[projectID] == nil {
		(*m)[projectID] = make(map[string]string)
	}
	(*m)[projectID][id] = hex
}

// HiddenStatesFor returns the collapsed state IDs of one project as a set.
func (c Config) HiddenStatesFor(projectID string) map[string]bool {
	out := make(map[string]bool)
	for _, id := range c.HiddenStates[projectID] {
		out[id] = true
	}
	return out
}

// SetHiddenStates records which of a project's state columns are collapsed. Passing an empty
// set drops the project's entry entirely rather than persisting an empty list.
func (c *Config) SetHiddenStates(projectID string, hidden map[string]bool) {
	ids := make([]string, 0, len(hidden))
	for id, on := range hidden {
		if on {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		delete(c.HiddenStates, projectID)
		return
	}
	sort.Strings(ids) // stable on disk, so saving twice does not churn the file
	if c.HiddenStates == nil {
		c.HiddenStates = make(map[string][]string)
	}
	c.HiddenStates[projectID] = ids
}

// Path returns ~/.config/plane-cli/config.json, honoring $XDG_CONFIG_HOME.
func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "plane-cli", "config.json"), nil
}

// Load reads the saved config. A missing file is not an error: it returns a zero Config.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Save writes the config, creating its directory if needed. The file holds an API token, so
// it is written with 0600 permissions.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
