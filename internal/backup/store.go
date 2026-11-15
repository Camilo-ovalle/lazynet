// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package backup stores running-config snapshots in a local git repository
// (pure Go, no git binary needed) so every backup is versioned and diffable.
package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Store is a git-backed backup directory.
type Store struct {
	dir  string
	repo *git.Repository
	mu   sync.Mutex // go-git worktrees are not safe for concurrent commits
}

// Entry is one backup (a commit touching the device's file).
type Entry struct {
	Hash    string    `json:"hash"`
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

const fileName = "running-config.cfg"

// Open opens or initialises the repository at dir.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	repo, err := git.PlainOpen(dir)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		repo, err = git.PlainInit(dir, false)
	}
	if err != nil {
		return nil, err
	}
	return &Store{dir: dir, repo: repo}, nil
}

// Dir is the repository path.
func (s *Store) Dir() string { return s.dir }

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// DeviceDir is the per-device folder name (sanitised label).
func DeviceDir(label string) string {
	n := strings.Trim(unsafeName.ReplaceAllString(label, "_"), "._")
	if n == "" {
		n = "device"
	}
	return n
}

// volatile lines change on every "show running-config" without any real
// configuration change; dropping them keeps diffs meaningful.
var volatile = regexp.MustCompile(`(?m)^(Building configuration\.\.\.|Current configuration : \d+ bytes|!Time: .*|ntp clock-period \d+)\s*\n`)

// Clean normalises a running-config for storage.
func Clean(cfg string) string {
	cfg = strings.ReplaceAll(cfg, "\r\n", "\n")
	cfg = volatile.ReplaceAllString(cfg, "")
	return strings.TrimLeft(cfg, "\n")
}

// Result describes a Save call.
type Result struct {
	Path    string `json:"path"`
	Hash    string `json:"hash"`
	Changed bool   `json:"changed"` // false = identical to the previous backup
}

// Save writes the config for label and commits it. An unchanged config still
// returns success but creates no new commit.
func (s *Store) Save(label, host, cfg string, when time.Time) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rel := filepath.ToSlash(filepath.Join(DeviceDir(label), fileName))
	abs := filepath.Join(s.dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o700); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(abs, []byte(Clean(cfg)), 0o600); err != nil {
		return Result{}, err
	}
	wt, err := s.repo.Worktree()
	if err != nil {
		return Result{}, err
	}
	if _, err := wt.Add(rel); err != nil {
		return Result{}, err
	}
	msg := fmt.Sprintf("backup %s (%s) %s", label, host, when.Format(time.RFC3339))
	hash, err := wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: "lazynet", Email: "lazynet@localhost", When: when},
	})
	if errors.Is(err, git.ErrEmptyCommit) {
		// Identical to the previous backup: nothing new to record.
		prev, lerr := s.list(label)
		if lerr != nil || len(prev) == 0 {
			return Result{}, fmt.Errorf("no changes but no previous backup found: %v", lerr)
		}
		return Result{Path: abs, Hash: prev[0].Hash, Changed: false}, nil
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Path: abs, Hash: hash.String(), Changed: true}, nil
}

// List returns the backups of label, newest first.
func (s *Store) List(label string) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.list(label)
}

func (s *Store) list(label string) ([]Entry, error) {
	rel := DeviceDir(label) + "/" + fileName
	if _, err := s.repo.Head(); err != nil {
		return nil, nil // empty repository
	}
	iter, err := s.repo.Log(&git.LogOptions{FileName: &rel})
	if err != nil {
		return nil, err
	}
	var out []Entry
	err = iter.ForEach(func(c *object.Commit) error {
		out = append(out, Entry{Hash: c.Hash.String(), Time: c.Author.When, Message: strings.TrimSpace(c.Message)})
		return nil
	})
	return out, err
}

// Content returns the config stored for label at a given commit.
func (s *Store) Content(label, hash string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.repo.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return "", err
	}
	f, err := c.File(DeviceDir(label) + "/" + fileName)
	if err != nil {
		return "", err
	}
	return f.Contents()
}

// Diff returns a unified diff of label's config between two commits
// (older → newer). Empty newer means the latest backup; empty older means the
// backup right before newer.
func (s *Store) Diff(label, older, newer string) (string, error) {
	entries, err := s.List(label)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("no backups for %s", label)
	}
	idx := func(h string) int {
		for i, e := range entries {
			if strings.HasPrefix(e.Hash, h) {
				return i
			}
		}
		return -1
	}
	ni := 0
	if newer != "" {
		if ni = idx(newer); ni < 0 {
			return "", fmt.Errorf("backup %s not found", newer)
		}
	}
	oi := ni + 1
	if older != "" {
		if oi = idx(older); oi < 0 {
			return "", fmt.Errorf("backup %s not found", older)
		}
	}
	if oi >= len(entries) {
		return "", fmt.Errorf("only one backup of %s: nothing to compare", label)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.repo.CommitObject(plumbing.NewHash(entries[oi].Hash))
	if err != nil {
		return "", err
	}
	b, err := s.repo.CommitObject(plumbing.NewHash(entries[ni].Hash))
	if err != nil {
		return "", err
	}
	patch, err := a.Patch(b)
	if err != nil {
		return "", err
	}
	rel := DeviceDir(label) + "/" + fileName
	var keep []diff.FilePatch
	for _, fp := range patch.FilePatches() {
		from, to := fp.Files()
		if (from != nil && from.Path() == rel) || (to != nil && to.Path() == rel) {
			keep = append(keep, fp)
		}
	}
	if len(keep) == 0 {
		return "", nil
	}
	var out strings.Builder
	if err := diff.NewUnifiedEncoder(&out, 3).Encode(filePatches(keep)); err != nil {
		return "", err
	}
	return out.String(), nil
}

// filePatches adapts a subset of file patches to diff.Patch.
type filePatches []diff.FilePatch

func (f filePatches) FilePatches() []diff.FilePatch { return f }
func (f filePatches) Message() string               { return "" }
