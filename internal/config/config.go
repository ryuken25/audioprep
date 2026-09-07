// Package config persists the handful of things worth remembering between
// launches as a small JSON file in the user's config directory.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config is the on-disk shape. Keep it flat and boring; every field is
// optional and a missing file is not an error.
type Config struct {
	LastPreset string `json:"last_preset,omitempty"`
	OutputDir  string `json:"output_dir,omitempty"`
	Theme      string `json:"theme,omitempty"` // "dark" or "light"
	// Advanced overrides the user last used with the Custom preset. Stored
	// as a free-form map so adding a knob never breaks old config files.
	Custom map[string]any `json:"custom,omitempty"`
}

// Path returns %APPDATA%\audioprep\config.json (or the OS equivalent).
func Path() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "audioprep", "config.json"), nil
}

// Load reads the config. A missing file returns an empty Config and no
// error; a corrupt file returns the error so the caller can decide (the app
// just logs it and continues with defaults).
func Load() (Config, error) {
	var c Config
	p, err := Path()
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c, nil
		}
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Save writes the config, creating the folder if needed. It writes to a temp
// file and renames so a crash mid-write cannot corrupt the config.
func Save(c Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
