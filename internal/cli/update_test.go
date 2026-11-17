// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Camilo-ovalle/lazynet/internal/update"
)

// runCLI runs lazynet with args and returns what it printed on stdout.
func runCLI(t *testing.T, version string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("LAZYNET_HOME", t.TempDir()) // never touch the user's files
	old := Version
	Version = version
	t.Cleanup(func() { Version = old })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	g := &globals{prompt: newPrompter()}
	root := newRoot(g)
	root.SetArgs(args)
	err = root.Execute()
	os.Stdout = stdout
	_ = w.Close()
	return <-done, err
}

func fakeGitHub(t *testing.T, tag string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repo/releases/latest" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": tag, "html_url": "https://example.invalid/" + tag})
	}))
	t.Cleanup(srv.Close)
	old := update.API
	update.API = srv.URL + "/repo"
	t.Cleanup(func() { update.API = old })
}

func TestUpdateCheck(t *testing.T) {
	fakeGitHub(t, "v0.3.0")

	out, err := runCLI(t, "0.2.0", "update", "--check", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var res struct {
		Current, Latest, URL string
		UpdateAvailable      bool `json:"update_available"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if res.Current != "0.2.0" || res.Latest != "0.3.0" || !res.UpdateAvailable || res.URL != "https://example.invalid/v0.3.0" {
		t.Fatalf("%+v", res)
	}

	out, err = runCLI(t, "0.2.0", "update", "--check")
	if err != nil || !strings.Contains(out, "lazynet 0.3.0 is available") || !strings.Contains(out, "lazynet update") {
		t.Fatalf("%v: %s", err, out)
	}
	out, err = runCLI(t, "0.3.0", "update", "--check")
	if err != nil || !strings.Contains(out, "0.3.0 is up to date") {
		t.Fatalf("%v: %s", err, out)
	}
	// Nothing newer: `lazynet update` stops before touching anything.
	out, err = runCLI(t, "0.3.0", "update")
	if err != nil || !strings.Contains(out, "0.3.0 is up to date") {
		t.Fatalf("%v: %s", err, out)
	}
	out, err = runCLI(t, "dev", "update", "--check")
	if err != nil || !strings.Contains(out, "development build") {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestUpdateRefuses(t *testing.T) {
	fakeGitHub(t, "v0.3.0")
	for _, c := range []struct {
		version string
		args    []string
		want    string
	}{
		{"dev", []string{"update"}, "development build"},
		{"v0.1.0-14-g6e12b9b", []string{"update"}, "development build"},
		{"0.2.0", []string{"--demo", "update"}, "not available in demo mode"},
		{"0.2.0", []string{"update", "-o", "json"}, "only supported with --check"},
		{"0.2.0", []string{"update", "-o", "yaml"}, "--output must be"},
	} {
		if _, err := runCLI(t, c.version, c.args...); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s %v: err = %v, want %q", c.version, c.args, err, c.want)
		}
	}
}

func TestChangelog(t *testing.T) {
	out, err := runCLI(t, "dev", "changelog", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var notes []update.Note
	if err := json.Unmarshal([]byte(out), &notes); err != nil || len(notes) == 0 {
		t.Fatalf("%v: %s", err, out)
	}
	if notes[len(notes)-1].Version != "0.1.0" {
		t.Fatalf("oldest release: %+v", notes[len(notes)-1])
	}

	out, err = runCLI(t, "0.1.0", "changelog")
	if err != nil || !strings.Contains(out, "lazynet 0.1.0") || !strings.Contains(out, "First release") {
		t.Fatalf("%v: %s", err, out)
	}
	out, err = runCLI(t, "0.1.0", "changelog", "--since", "0.1.0")
	if err != nil || !strings.Contains(out, "No release notes to show") {
		t.Fatalf("%v: %s", err, out)
	}
	out, err = runCLI(t, "0.1.0", "changelog", "--since", "0.1.0", "-o", "json")
	if err != nil || strings.TrimSpace(out) != "[]" {
		t.Fatalf("%v: %q", err, out)
	}
	if _, err := runCLI(t, "0.1.0", "changelog", "--since", "latest"); err == nil {
		t.Fatal("--since must be a version")
	}
}

// The UI only announces releases in real release builds.
func TestUpdatesDisabled(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })
	Version = "0.2.0"
	if (&globals{demo: true}).updates() != nil {
		t.Fatal("demo mode must not check for updates")
	}
	Version = "dev"
	if (&globals{}).updates() != nil {
		t.Fatal("development builds must not check for updates")
	}
}
