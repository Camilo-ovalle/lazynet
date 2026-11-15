// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package config resolves lazynet's on-disk locations and user configuration.
package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const appName = "lazynet"

// Paths holds every location lazynet reads from or writes to.
type Paths struct {
	ConfigDir  string // config.yml, inventory.yml, known_hosts
	DataDir    string // backups/, audit.log
	CacheDir   string // detection cache
	Config     string
	Inventory  string
	KnownHosts string
	Backups    string
	AuditLog   string
	Detect     string
}

// DefaultPaths follows the XDG base directory spec on Linux and the native
// equivalents on other platforms. LAZYNET_HOME overrides everything (useful
// for tests and demo mode).
func DefaultPaths() (Paths, error) {
	if home := os.Getenv("LAZYNET_HOME"); home != "" {
		return PathsUnder(home), nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return Paths{}, err
	}
	data, err := userDataDir()
	if err != nil {
		return Paths{}, err
	}
	return build(filepath.Join(cfg, appName), filepath.Join(data, appName), filepath.Join(cache, appName)), nil
}

// PathsUnder places every lazynet file under a single root directory.
func PathsUnder(root string) Paths {
	return build(filepath.Join(root, "config"), filepath.Join(root, "data"), filepath.Join(root, "cache"))
}

func build(cfg, data, cache string) Paths {
	return Paths{
		ConfigDir:  cfg,
		DataDir:    data,
		CacheDir:   cache,
		Config:     filepath.Join(cfg, "config.yml"),
		Inventory:  filepath.Join(cfg, "inventory.yml"),
		KnownHosts: filepath.Join(cfg, "known_hosts"),
		Backups:    filepath.Join(data, "backups"),
		AuditLog:   filepath.Join(data, "audit.log"),
		Detect:     filepath.Join(cache, "detected.json"),
	}
}

func userDataDir() (string, error) {
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return d, nil
		}
		return os.UserConfigDir()
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

// Ensure creates the directories lazynet needs, with private permissions.
func (p Paths) Ensure() error {
	for _, d := range []string{p.ConfigDir, p.DataDir, p.CacheDir, p.Backups} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return err
		}
	}
	return nil
}
