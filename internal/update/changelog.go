// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package update

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Camilo-ovalle/lazynet"
)

// Note is the changelog section of one release.
type Note struct {
	Version string `json:"version"`
	Date    string `json:"date,omitempty"`
	Body    string `json:"notes"`
}

// heading matches "## [0.2.0] - 2026-11-20", "## 0.2.0" and "## [Unreleased]".
var heading = regexp.MustCompile(`^##\s+\[?([^\]\s]+)\]?(?:\s+-\s+(\S+))?\s*$`)

// ParseChangelog reads the release sections of a Keep a Changelog style
// file. The Unreleased section is skipped. Versions must be valid, unique and
// listed newest first, so a mistake is caught by the tests instead of
// showing users the wrong notes.
func ParseChangelog(md string) ([]Note, error) {
	var notes []Note
	var cur *Note
	var body []string
	flush := func() {
		if cur != nil {
			cur.Body = strings.TrimSpace(strings.Join(body, "\n"))
			notes = append(notes, *cur)
		}
		cur, body = nil, nil
	}
	for i, line := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		if !strings.HasPrefix(line, "## ") {
			if cur != nil {
				body = append(body, line)
			}
			continue
		}
		flush()
		m := heading.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("changelog line %d: unexpected heading %q", i+1, line)
		}
		if strings.EqualFold(m[1], "unreleased") {
			continue // its lines are dropped: cur is nil
		}
		if !IsRelease(m[1]) {
			return nil, fmt.Errorf("changelog line %d: %q is not a release version (X.Y.Z)", i+1, m[1])
		}
		v, _ := ParseVersion(m[1])
		if len(notes) > 0 && Compare(v.String(), notes[len(notes)-1].Version) >= 0 {
			return nil, fmt.Errorf("changelog line %d: %s must be older than %s, which is listed above it", i+1, v, notes[len(notes)-1].Version)
		}
		cur = &Note{Version: v.String(), Date: m[2]}
	}
	flush()
	return notes, nil
}

// Notes returns the release notes embedded in this binary, newest first.
func Notes() []Note {
	notes, err := ParseChangelog(lazynet.Changelog)
	if err != nil {
		return nil // TestEmbeddedChangelog keeps this from happening
	}
	return notes
}

// Between returns the notes of the versions newer than after, up to and
// including upTo, newest first. An empty after means "only upTo".
func Between(notes []Note, after, upTo string) []Note {
	var out []Note
	for _, n := range notes {
		if Compare(n.Version, upTo) > 0 {
			continue
		}
		if after == "" {
			if Compare(n.Version, upTo) == 0 {
				out = append(out, n)
			}
			continue
		}
		if Compare(n.Version, after) > 0 {
			out = append(out, n)
		}
	}
	return out
}
