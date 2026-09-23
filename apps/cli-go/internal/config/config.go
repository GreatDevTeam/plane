// Package config persists the last-used server connection so plane-cli does not need to
// sign in again on every run.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is the on-disk connection profile.
type Config struct {
	ServerURL     string `json:"server_url"`
	Token         string `json:"token"`
	WorkspaceSlug string `json:"workspace_slug"`
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
