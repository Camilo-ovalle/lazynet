// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package update

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Camilo-ovalle/lazynet"
)

func TestParseAndCompare(t *testing.T) {
	for _, s := range []string{"0.1.0", "v0.1.0", "10.20.30", "1.0.0-rc.1"} {
		if _, ok := ParseVersion(s); !ok {
			t.Errorf("ParseVersion(%q) failed", s)
		}
	}
	for _, s := range []string{"", "dev", "1.2", "1.2.3.4", "01.2.3", "a.b.c", "1.-2.3"} {
		if _, ok := ParseVersion(s); ok {
			t.Errorf("ParseVersion(%q) should fail", s)
		}
	}
	for _, s := range []string{"0.1.0", "v1.2.3"} {
		if !IsRelease(s) {
			t.Errorf("IsRelease(%q) = false", s)
		}
	}
	// Builds that are not releases: plain go build, make build after a tag,
	// go install @main.
	for _, s := range []string{"dev", "v0.1.0-14-g6e12b9b", "0.1.1-0.20261116120000-6e12b9b0c1d2", "0.2.0-rc.1"} {
		if IsRelease(s) {
			t.Errorf("IsRelease(%q) = true", s)
		}
	}
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.2.0", -1}, {"0.10.0", "0.9.0", 1}, {"1.0.0", "v1.0.0", 0},
		{"1.0.0-rc.1", "1.0.0", -1}, {"1.0.0", "1.0.0-rc.1", 1}, {"dev", "0.0.1", -1},
		{"2.0.0", "1.99.99", 1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// The embedded changelog must always parse: the release workflow and the
// update banner depend on it.
func TestEmbeddedChangelog(t *testing.T) {
	notes, err := ParseChangelog(lazynet.Changelog)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) == 0 || len(Notes()) != len(notes) {
		t.Fatalf("no release notes embedded: %d", len(notes))
	}
	for _, n := range notes {
		if n.Body == "" {
			t.Errorf("release %s has no notes", n.Version)
		}
		if _, err := time.Parse("2006-01-02", n.Date); err != nil {
			t.Errorf("release %s: date %q is not YYYY-MM-DD", n.Version, n.Date)
		}
	}
	if !strings.Contains(lazynet.Changelog, "## [Unreleased]") {
		t.Error("CHANGELOG.md must keep an [Unreleased] section for the next release")
	}
}

const sampleChangelog = `# Changelog

Intro text.

## [Unreleased]

- not released yet

## [0.3.0] - 2026-12-01

### Added

- three

## [0.2.0] - 2026-11-20

- two

## 0.1.0

- one
`

func TestParseChangelog(t *testing.T) {
	notes, err := ParseChangelog(sampleChangelog)
	if err != nil {
		t.Fatal(err)
	}
	var vs []string
	for _, n := range notes {
		vs = append(vs, n.Version+"|"+n.Date+"|"+n.Body)
	}
	want := []string{"0.3.0|2026-12-01|### Added\n\n- three", "0.2.0|2026-11-20|- two", "0.1.0||- one"}
	if strings.Join(vs, ",") != strings.Join(want, ",") {
		t.Fatalf("notes = %q", vs)
	}
	for name, bad := range map[string]string{
		"order":     "## [0.1.0] - 2026-01-01\n\n## [0.2.0] - 2026-01-02\n",
		"duplicate": "## [0.2.0]\n\n## [0.2.0]\n",
		"version":   "## [next]\n",
		"pre":       "## [1.0.0-rc.1]\n",
	} {
		if _, err := ParseChangelog(bad); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	between := func(after, upTo string) string {
		var out []string
		for _, n := range Between(notes, after, upTo) {
			out = append(out, n.Version)
		}
		return strings.Join(out, ",")
	}
	for _, c := range [][3]string{
		{"", "0.2.0", "0.2.0"},
		{"0.1.0", "0.3.0", "0.3.0,0.2.0"},
		{"0.2.0", "0.2.0", ""},
		{"0.1.0", "0.2.0", "0.2.0"},
		{"", "0.4.0", ""},
	} {
		if got := between(c[0], c[1]); got != c[2] {
			t.Errorf("Between(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func versions(ns []Note) string {
	var out []string
	for _, n := range ns {
		out = append(out, n.Version)
	}
	return strings.Join(out, ",")
}

func TestWhatsNew(t *testing.T) {
	notes, _ := ParseChangelog(sampleChangelog)
	path := t.TempDir() + "/update.json"
	c := &Checker{Version: "0.2.0", StatePath: path, Notes: notes}

	// Nothing recorded (first run, or updated from a version that didn't
	// record it): the notes of the running version.
	if got := versions(c.WhatsNew()); got != "0.2.0" {
		t.Fatalf("first run: %q", got)
	}
	if got := c.WhatsNew(); got != nil {
		t.Fatalf("shown twice: %v", got)
	}
	if LoadState(path).LastSeen != "0.2.0" {
		t.Fatalf("state: %+v", LoadState(path))
	}

	// Updated from 0.1.0 to 0.3.0: both new releases.
	_ = SaveState(path, State{LastSeen: "0.1.0", Skipped: "0.2.0"})
	c.Version = "0.3.0"
	if got := versions(c.WhatsNew()); got != "0.3.0,0.2.0" {
		t.Fatalf("upgrade: %q", got)
	}
	if st := LoadState(path); st.LastSeen != "0.3.0" || st.Skipped != "0.2.0" {
		t.Fatalf("state must keep other fields: %+v", st)
	}

	// Downgrade: nothing to announce, but the running version is recorded.
	c.Version = "0.2.0"
	if got := c.WhatsNew(); got != nil {
		t.Fatalf("downgrade: %v", got)
	}

	// Development builds never show notes or touch the state.
	c.Version = "dev"
	if c.WhatsNew() != nil || LoadState(path).LastSeen != "0.2.0" {
		t.Fatal("dev builds must be ignored")
	}

	// CLI updates print the notes and record them.
	if err := MarkSeen(path, "0.3.0"); err != nil || LoadState(path).LastSeen != "0.3.0" {
		t.Fatalf("MarkSeen: %v %+v", err, LoadState(path))
	}
	_ = MarkSeen(path, "0.1.0")
	if LoadState(path).LastSeen != "0.3.0" {
		t.Fatal("MarkSeen must never go back")
	}
}

func TestCheck(t *testing.T) {
	srv := newReleaseServer(t, "0.3.0", nil)
	path := t.TempDir() + "/update.json"
	now := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	c := &Checker{Version: "0.2.0", StatePath: path, Client: srv.client(), Online: true, Now: func() time.Time { return now }}
	ctx := context.Background()

	av, err := c.Check(ctx)
	if err != nil || av.Version != "0.3.0" || av.Current != "0.2.0" || !strings.Contains(av.URL, "v0.3.0") {
		t.Fatalf("Check = %+v, %v", av, err)
	}
	if srv.apiCalls != 1 {
		t.Fatalf("api calls = %d", srv.apiCalls)
	}
	// Within the interval the cached answer is used.
	now = now.Add(23 * time.Hour)
	if av, _ := c.Check(ctx); av.Version != "0.3.0" || srv.apiCalls != 1 {
		t.Fatalf("cached: %+v, calls %d", av, srv.apiCalls)
	}
	// Skipped: not offered again, until something newer appears.
	if err := c.Skip("0.3.0"); err != nil {
		t.Fatal(err)
	}
	if av, _ := c.Check(ctx); av.Version != "" {
		t.Fatalf("skipped version offered: %+v", av)
	}
	now = now.Add(2 * time.Hour)
	srv.version = "0.4.0"
	if av, _ := c.Check(ctx); av.Version != "0.4.0" || srv.apiCalls != 2 {
		t.Fatalf("after the interval: %+v, calls %d", av, srv.apiCalls)
	}
	// A clock that went backwards refreshes instead of trusting the cache.
	now = now.Add(-48 * time.Hour)
	_, _ = c.Check(ctx)
	if srv.apiCalls != 3 {
		t.Fatalf("calls = %d", srv.apiCalls)
	}

	// Up to date.
	c.Version = "0.4.0"
	if av, _ := c.Check(ctx); av.Version != "" {
		t.Fatalf("up to date: %+v", av)
	}

	// check_updates: false and dev builds never use the network.
	calls := srv.apiCalls
	off := &Checker{Version: "0.1.0", StatePath: t.TempDir() + "/u.json", Client: srv.client(), Online: false}
	dev := &Checker{Version: "dev", StatePath: t.TempDir() + "/u.json", Client: srv.client(), Online: true}
	for _, ch := range []*Checker{off, dev} {
		if av, err := ch.Check(ctx); av.Version != "" || err != nil {
			t.Fatalf("%s: %+v %v", ch.Version, av, err)
		}
	}
	if srv.apiCalls != calls {
		t.Fatal("offline checks must not contact GitHub")
	}

	// Errors are reported and not cached.
	srv.fail = true
	bad := &Checker{Version: "0.1.0", StatePath: t.TempDir() + "/u.json", Client: srv.client(), Online: true}
	if _, err := bad.Check(ctx); err == nil {
		t.Fatal("expected an error")
	}
	if !LoadState(bad.StatePath).CheckedAt.IsZero() {
		t.Fatal("a failed check must not be cached")
	}
	if err := c.Skip("dev"); err == nil {
		t.Fatal("Skip must refuse non-release versions")
	}
}

func TestLatestRejectsPreReleases(t *testing.T) {
	srv := newReleaseServer(t, "0.3.0-rc.1", nil)
	if _, err := srv.client().Latest(context.Background()); err == nil {
		t.Fatal("a pre-release must not be offered")
	}
}

func TestStateFile(t *testing.T) {
	path := t.TempDir() + "/sub/update.json"
	if LoadState(path) != (State{}) {
		t.Fatal("missing file must be an empty state")
	}
	if err := SaveState(path, State{LastSeen: "0.1.0"}); err != nil {
		t.Fatal(err)
	}
	if LoadState(path).LastSeen != "0.1.0" {
		t.Fatal("state not saved")
	}
	assertMode(t, path, 0o600)
}
