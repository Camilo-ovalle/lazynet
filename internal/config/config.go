// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the user configuration stored in config.yml.
type Config struct {
	// CredentialTTL is the grace period during which an entered password is
	// reused (like sudo). 0 disables caching entirely.
	CredentialTTL Duration `yaml:"credential_ttl"`
	// Workers bounds concurrent SSH sessions for group operations.
	Workers int `yaml:"workers"`
	// SSHTimeout is the TCP/SSH connection timeout.
	SSHTimeout Duration `yaml:"ssh_timeout"`
	// CommandTimeout bounds a single command (show running-config can be slow).
	CommandTimeout Duration `yaml:"command_timeout"`
	// DefaultUsername is used when neither -u nor the inventory provide one.
	DefaultUsername string `yaml:"default_username"`
	// LegacySSH enables old ciphers/kex (CBC, group14-sha1, ssh-rsa) needed by
	// older IOS 12.x/15.x Catalyst switches. Modern algorithms are still preferred.
	LegacySSH bool `yaml:"legacy_ssh"`
}

// Default returns the configuration used when no config.yml exists.
func Default() Config {
	return Config{
		CredentialTTL:  Duration(5 * time.Minute),
		Workers:        5,
		SSHTimeout:     Duration(15 * time.Second),
		CommandTimeout: Duration(90 * time.Second),
		LegacySSH:      true,
	}
}

// Load reads config.yml, falling back to defaults for missing fields.
func Load(path string) (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	return cfg, nil
}

// Duration is a time.Duration that marshals as "5m", "30s", etc.
type Duration time.Duration

// D returns the value as a time.Duration.
func (d Duration) D() time.Duration { return time.Duration(d) }

// UnmarshalYAML accepts Go duration strings.
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	v, err := time.ParseDuration(n.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", n.Value, err)
	}
	*d = Duration(v)
	return nil
}

// MarshalYAML renders the duration as a string.
func (d Duration) MarshalYAML() (any, error) { return time.Duration(d).String(), nil }
