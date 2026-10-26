package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Config is persisted in the user's configuration directory. It remembers the
// API endpoint of every wiki host seen so far, and the bot credentials of the
// wikis the user logged in to.
type Config struct {
	// Wikis is keyed by API endpoint URL.
	Wikis map[string]*Wiki `json:"wikis"`
	// Hosts maps a host name as typed by the user to an API endpoint URL.
	Hosts map[string]string `json:"hosts"`

	path string
}

type Wiki struct {
	API         string `json:"-"`
	Server      string `json:"server"`
	ArticlePath string `json:"articlepath"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
}

func configPath() (string, error) {
	if p := os.Getenv("MEDIAWIKI_CLI_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mediawiki-cli", "config.json"), nil
}

func loadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	cfg := &Config{Wikis: map[string]*Wiki{}, Hosts: map[string]string{}, path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if cfg.Wikis == nil {
		cfg.Wikis = map[string]*Wiki{}
	}
	if cfg.Hosts == nil {
		cfg.Hosts = map[string]string{}
	}
	for api, w := range cfg.Wikis {
		w.API = api
	}
	return cfg, nil
}

// save writes the configuration readable by the current user only, since it
// holds passwords.
func (c *Config) save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}
