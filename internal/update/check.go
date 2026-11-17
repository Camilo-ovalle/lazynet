// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package update

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// State is what lazynet remembers between runs (update.json in the data
// directory). It holds no secrets.
type State struct {
	// LastSeen is the newest version whose release notes were shown.
	LastSeen string `json:"last_seen_version,omitempty"`
	// Skipped is a version the user chose not to install. Newer ones are
	// offered again.
	Skipped string `json:"skipped_version,omitempty"`
	// CheckedAt, Latest and LatestURL cache the last answer from GitHub.
	CheckedAt time.Time `json:"checked_at,omitzero"`
	Latest    string    `json:"latest_version,omitempty"`
	LatestURL string    `json:"latest_url,omitempty"`
}

// LoadState reads the state file. A missing or unreadable file is an empty
// state: forgetting it only means seeing a banner again.
func LoadState(path string) State {
	var s State
	b, err := os.ReadFile(path)
	if err != nil {
		return State{}
	}
	if json.Unmarshal(b, &s) != nil {
		return State{}
	}
	return s
}

// SaveState writes the state file atomically, readable only by the user.
func SaveState(path string, s State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".update-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// DefaultInterval is how long an answer from GitHub is reused.
const DefaultInterval = 24 * time.Hour

// Checker answers the two questions the terminal UI asks when it opens:
// "which release notes haven't been shown yet?" and "is there a newer
// release?".
type Checker struct {
	Version   string // running version; must be a release (IsRelease)
	StatePath string
	Client    Client
	// Online allows contacting GitHub. When false (check_updates: false),
	// Check never uses the network, but release notes are still shown.
	Online   bool
	Interval time.Duration
	Now      func() time.Time
	// Notes defaults to the changelog embedded in the binary.
	Notes []Note
}

// Available describes a release newer than the running one.
type Available struct {
	Current string
	Version string
	URL     string
}

func (c *Checker) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Checker) notes() []Note {
	if c.Notes != nil {
		return c.Notes
	}
	return Notes()
}

// WhatsNew returns the release notes the user hasn't seen yet, newest first,
// and records them as seen. After an update from 0.2.0 to 0.4.0 it returns
// the notes of 0.4.0 and 0.3.0. When nothing was recorded yet (first run, or
// an update from a version that didn't record it) it returns the notes of
// the running version only.
func (c *Checker) WhatsNew() []Note {
	if !IsRelease(c.Version) {
		return nil
	}
	st := LoadState(c.StatePath)
	if st.LastSeen == c.Version {
		return nil
	}
	prev := st.LastSeen
	if !IsRelease(prev) {
		prev = ""
	}
	var out []Note
	if prev == "" || Compare(prev, c.Version) < 0 { // a downgrade shows nothing
		out = Between(c.notes(), prev, c.Version)
	}
	st.LastSeen = c.Version
	_ = SaveState(c.StatePath, st)
	return out
}

// Check reports whether a release newer than the running one exists and
// wasn't skipped. It reuses GitHub's last answer for Interval, so opening
// the UI many times a day makes one request.
func (c *Checker) Check(ctx context.Context) (Available, error) {
	if !c.Online || !IsRelease(c.Version) {
		return Available{}, nil
	}
	st := LoadState(c.StatePath)
	interval := c.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	age := c.now().Sub(st.CheckedAt)
	if st.Latest == "" || age < 0 || age >= interval {
		rel, err := c.Client.Latest(ctx)
		if err != nil {
			return Available{}, err
		}
		st.CheckedAt, st.Latest, st.LatestURL = c.now(), rel.Version, rel.URL
		_ = SaveState(c.StatePath, st)
	}
	if Compare(st.Latest, c.Version) <= 0 || st.Skipped == st.Latest {
		return Available{}, nil
	}
	return Available{Current: c.Version, Version: st.Latest, URL: st.LatestURL}, nil
}

// Skip stops offering version v. A newer release is offered again.
func (c *Checker) Skip(v string) error {
	if !IsRelease(v) {
		return errors.New("not a release version: " + v)
	}
	st := LoadState(c.StatePath)
	st.Skipped = v
	return SaveState(c.StatePath, st)
}

// MarkSeen records that the notes up to v were shown (the CLI prints them
// right after `lazynet update`).
func MarkSeen(path, v string) error {
	st := LoadState(path)
	if IsRelease(st.LastSeen) && Compare(st.LastSeen, v) >= 0 {
		return nil
	}
	st.LastSeen = v
	return SaveState(path, st)
}
