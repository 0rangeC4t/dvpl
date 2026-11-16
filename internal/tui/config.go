package tui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

type userConfig struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Level       int    `json:"level"`
	Workers     int    `json:"workers"`
	Language    string `json:"language"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "dvpl", "config.json"), nil
}

func loadConfig(path string) (userConfig, error) {
	var cfg userConfig
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return userConfig{}, err
	}
	if cfg.Level < 0 || cfg.Level > 2 || cfg.Workers < 1 || cfg.Workers > 64 || (cfg.Language != "vi" && cfg.Language != "en") {
		return userConfig{}, errors.New("invalid DVPL config values")
	}
	return cfg, nil
}

func saveConfig(path string, cfg userConfig) error {
	if cfg.Level < 0 || cfg.Level > 2 || cfg.Workers < 1 || cfg.Workers > 64 || (cfg.Language != "vi" && cfg.Language != "en") {
		return errors.New("invalid DVPL config values")
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".dvpl-config-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return errors.Join(err, closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, path)
}

func (m *Model) loadConfig() {
	path, err := configPath()
	if err != nil {
		return
	}
	cfg, err := loadConfig(path)
	if err != nil {
		return
	}
	m.inputs[0].SetValue(cfg.Source)
	m.inputs[1].SetValue(cfg.Destination)
	m.inputs[2].SetValue(strconv.Itoa(cfg.Workers))
	m.level, m.locale = cfg.Level, cfg.Language
	// NewModel applies the saved locale before rendering text.
}

func (m *Model) saveConfig() {
	path, err := configPath()
	if err != nil {
		return
	}
	workers, err := strconv.Atoi(m.inputs[2].Value())
	if err != nil || workers < 1 || workers > 64 {
		return
	}
	locale := m.locale
	if locale != "vi" && locale != "en" {
		locale = "vi"
	}
	_ = saveConfig(path, userConfig{Source: m.inputs[0].Value(), Destination: m.inputs[1].Value(), Level: m.level, Workers: workers, Language: locale})
}
