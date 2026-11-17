// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// releaseServer mimics the GitHub API and release downloads.
type releaseServer struct {
	*httptest.Server
	mu       sync.Mutex
	version  string
	files    map[string][]byte // asset name -> content
	corrupt  string            // asset served with content that doesn't match checksums.txt
	fail     bool
	apiCalls int
	agents   []string
}

func newReleaseServer(t *testing.T, version string, files map[string][]byte) *releaseServer {
	t.Helper()
	s := &releaseServer{version: version, files: files}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

func (s *releaseServer) client() Client {
	return Client{API: s.URL + "/repo", UserAgent: "lazynet/test", HTTP: s.Client()}
}

func (s *releaseServer) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents = append(s.agents, r.UserAgent())
	if s.fail {
		http.Error(w, "rate limited", http.StatusForbidden)
		return
	}
	switch {
	case r.URL.Path == "/repo/releases/latest":
		s.apiCalls++
		type asset struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}
		assets := []asset{{"checksums.txt", s.URL + "/dl/checksums.txt"}}
		for name := range s.files {
			assets = append(assets, asset{name, s.URL + "/dl/" + name})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v" + s.version, "html_url": "https://example.invalid/releases/tag/v" + s.version,
			"body": "notes", "prerelease": strings.Contains(s.version, "-"), "assets": assets,
		})
	case r.URL.Path == "/dl/checksums.txt":
		for name, b := range s.files {
			sum := sha256.Sum256(b)
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
		}
	case strings.HasPrefix(r.URL.Path, "/dl/"):
		name := strings.TrimPrefix(r.URL.Path, "/dl/")
		b, ok := s.files[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if name == s.corrupt {
			b = append([]byte("tampered"), b...)
		}
		_, _ = w.Write(b)
	default:
		http.NotFound(w, r)
	}
}

// fakeBinary is a script that answers `lazynet version` like the real one.
func fakeBinary(version string) []byte {
	return []byte("#!/bin/sh\necho 'lazynet " + version + " (commit abc1234, built 2026-11-20T00:00:00Z)'\n")
}

func tarball(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, b := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(b)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write(b)
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func release(t *testing.T, srv *releaseServer) Release {
	t.Helper()
	rel, err := srv.client().Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != want {
		t.Fatalf("%s: mode %v, want %v", path, fi.Mode().Perm(), want)
	}
}

func runOutput(name string, args ...string) (string, error) {
	b, err := exec.Command(name, args...).Output()
	return string(b), err
}

func newBinaryInstaller(t *testing.T, srv *releaseServer, oldVersion string) (*Installer, *bytes.Buffer) {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "lazynet")
	if err := os.WriteFile(exe, fakeBinary(oldVersion), 0o755); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	return &Installer{
		Client: srv.client(), Exe: exe, Out: out, Arch: "amd64",
		Run: func(name string, args ...string) error {
			t.Fatalf("unexpected command: %s %v", name, args)
			return nil
		},
		Output:   runOutput,
		LookPath: func(string) (string, error) { return "/usr/bin/sudo", nil },
	}, out
}

func TestInstallBinary(t *testing.T) {
	srv := newReleaseServer(t, "0.3.0", map[string][]byte{
		"lazynet_0.3.0_linux_amd64.tar.gz": tarball(t, map[string][]byte{"lazynet": fakeBinary("0.3.0"), "LICENSE": []byte("x")}),
	})
	in, out := newBinaryInstaller(t, srv, "0.2.0")
	if in.Method() != MethodBinary {
		t.Fatalf("method = %v", in.Method())
	}
	if err := in.Install(context.Background(), release(t, srv)); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	got, _ := runOutput(in.Exe, "version")
	if !strings.HasPrefix(got, "lazynet 0.3.0 ") {
		t.Fatalf("binary not replaced: %q", got)
	}
	assertMode(t, in.Exe, 0o755)
	if !strings.Contains(out.String(), "Checksum verified") {
		t.Fatalf("output: %s", out)
	}
	entries, _ := os.ReadDir(filepath.Dir(in.Exe))
	if len(entries) != 1 {
		t.Fatalf("temporary files left next to the binary: %v", entries)
	}
	for _, ua := range srv.agents {
		if ua != "lazynet/test" {
			t.Fatalf("user agent %q", ua)
		}
	}
}

func TestInstallRefusesBadDownloads(t *testing.T) {
	good := tarball(t, map[string][]byte{"lazynet": fakeBinary("0.3.0")})
	cases := map[string]struct {
		files   map[string][]byte
		corrupt string
		arch    string
		want    string
	}{
		"checksum": {map[string][]byte{"lazynet_0.3.0_linux_amd64.tar.gz": good}, "lazynet_0.3.0_linux_amd64.tar.gz", "amd64", "checksum mismatch"},
		"missing":  {map[string][]byte{"lazynet_0.3.0_linux_amd64.tar.gz": good}, "", "arm64", "has no lazynet_0.3.0_linux_arm64.tar.gz"},
		"no binary": {map[string][]byte{
			"lazynet_0.3.0_linux_amd64.tar.gz": tarball(t, map[string][]byte{"README.md": []byte("x")}),
		}, "", "amd64", "does not contain the lazynet binary"},
		"wrong version": {map[string][]byte{
			"lazynet_0.3.0_linux_amd64.tar.gz": tarball(t, map[string][]byte{"lazynet": fakeBinary("0.2.9")}),
		}, "", "amd64", "instead of lazynet 0.3.0"},
		"arch": {nil, "", "386", "no lazynet release for CPU architecture 386"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			srv := newReleaseServer(t, "0.3.0", c.files)
			srv.corrupt = c.corrupt
			in, _ := newBinaryInstaller(t, srv, "0.2.0")
			in.Arch = c.arch
			err := in.Install(context.Background(), release(t, srv))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
			if name == "wrong version" {
				return // the binary was replaced, and the check caught it
			}
			if got, _ := runOutput(in.Exe, "version"); !strings.HasPrefix(got, "lazynet 0.2.0 ") {
				t.Fatalf("the old binary must be untouched: %q", got)
			}
		})
	}
	t.Run("no checksums", func(t *testing.T) {
		in := &Installer{Arch: "amd64", Exe: "/nonexistent/lazynet", Out: &bytes.Buffer{}}
		rel := Release{Version: "0.3.0", Assets: map[string]string{"lazynet_0.3.0_linux_amd64.tar.gz": "http://127.0.0.1:1/x"}}
		if err := in.Install(context.Background(), rel); err == nil || !strings.Contains(err.Error(), "unverified") {
			t.Fatalf("err = %v", err)
		}
	})
}

// The install script puts lazynet in /usr/local/bin with sudo: the new
// binary is verified as the user and installed with sudo.
func TestInstallBinaryWithSudo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	srv := newReleaseServer(t, "0.3.0", map[string][]byte{
		"lazynet_0.3.0_linux_amd64.tar.gz": tarball(t, map[string][]byte{"lazynet": fakeBinary("0.3.0")}),
	})
	in, _ := newBinaryInstaller(t, srv, "0.2.0")
	dir := filepath.Dir(in.Exe)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	var ran []string
	in.Run = func(name string, args ...string) error {
		ran = append(ran, name+" "+strings.Join(args, " "))
		// The file handed to sudo is the verified new binary.
		if b, err := os.ReadFile(args[len(args)-2]); err != nil || !bytes.Equal(b, fakeBinary("0.3.0")) {
			t.Errorf("sudo got the wrong file: %v", err)
		}
		return nil
	}
	in.Output = func(string, ...string) (string, error) { return "lazynet 0.3.0 (commit x, built y)\n", nil }
	if err := in.Install(context.Background(), release(t, srv)); err != nil {
		t.Fatal(err)
	}
	if len(ran) != 1 || !strings.HasPrefix(ran[0], "sudo install -m 0755 ") || !strings.HasSuffix(ran[0], " "+in.Exe) {
		t.Fatalf("commands = %q", ran)
	}

	in.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	if err := in.Install(context.Background(), release(t, srv)); err == nil || !strings.Contains(err.Error(), "sudo is not installed") {
		t.Fatalf("err = %v", err)
	}
}

func TestInstallPackages(t *testing.T) {
	deb, rpm := []byte("deb package"), []byte("rpm package")
	srv := newReleaseServer(t, "0.3.0", map[string][]byte{
		"lazynet_0.3.0_linux_amd64.deb": deb,
		"lazynet_0.3.0_linux_amd64.rpm": rpm,
	})
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	list := write("lazynet.list", "deb [signed-by=/usr/share/keyrings/lazynet.gpg] https://camilo-ovalle.github.io/lazynet stable main\n")
	commented := write("old.list", "# deb https://camilo-ovalle.github.io/lazynet stable main\n")
	other := write("other.list", "deb http://deb.debian.org/debian trixie main\n")

	cases := []struct {
		name    string
		owner   string // which package database owns /usr/bin/lazynet
		sources []string
		root    bool
		reports string // `lazynet version` after the update
		method  Method
		want    []string
		err     string
		hint    bool // the retired APT source must be pointed out
	}{
		{"deb", "dpkg", []string{commented, other}, false, "0.3.0", MethodDeb, []string{"sudo dpkg -i lazynet_0.3.0_linux_amd64.deb"}, "", false},
		{"deb as root", "dpkg", nil, true, "0.3.0", MethodDeb, []string{"dpkg -i lazynet_0.3.0_linux_amd64.deb"}, "", false},
		// Installed from the APT repository of 0.1.0: updated as a .deb, and
		// told to remove the source that no longer exists.
		{"deb from the retired APT repository", "dpkg", []string{other, list}, false, "0.3.0", MethodDeb, []string{"sudo dpkg -i lazynet_0.3.0_linux_amd64.deb"}, "", true},
		{"rpm", "rpm", nil, false, "0.3.0", MethodRPM, []string{"sudo rpm -U lazynet_0.3.0_linux_amd64.rpm"}, "", false},
		{"package not upgraded", "rpm", nil, false, "0.2.0", MethodRPM, nil, "instead of lazynet 0.3.0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var ran []string
			out := &bytes.Buffer{}
			in := &Installer{
				Client: srv.client(), Exe: PackagePath, Out: out, Arch: "amd64", Root: c.root, Sources: c.sources,
				LookPath: func(string) (string, error) { return "/usr/bin/sudo", nil },
				Run: func(name string, args ...string) error {
					if n := len(args); n > 0 && (strings.HasSuffix(args[n-1], ".deb") || strings.HasSuffix(args[n-1], ".rpm")) {
						b, err := os.ReadFile(args[n-1])
						if err != nil || (!bytes.Equal(b, deb) && !bytes.Equal(b, rpm)) {
							t.Errorf("package not downloaded and verified: %v", err)
						}
						args[n-1] = filepath.Base(args[n-1])
					}
					ran = append(ran, strings.TrimSpace(name+" "+strings.Join(args, " ")))
					return nil
				},
				Output: func(name string, args ...string) (string, error) {
					switch name {
					case "dpkg-query", "rpm":
						if (name == "dpkg-query") == (c.owner == "dpkg") {
							return "lazynet: /usr/bin/lazynet\n", nil
						}
						return "", errors.New("not owned")
					case PackagePath:
						return "lazynet " + c.reports + " (commit x, built y)\n", nil
					}
					return "", fmt.Errorf("unexpected %s", name)
				},
			}
			if m := in.Method(); m != c.method {
				t.Fatalf("method = %v, want %v", m, c.method)
			}
			err := in.Install(context.Background(), release(t, srv))
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("err = %v, want %q", err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(ran, "\n") != strings.Join(c.want, "\n") {
				t.Fatalf("commands:\n%s\nwant:\n%s", strings.Join(ran, "\n"), strings.Join(c.want, "\n"))
			}
			if hint := strings.Contains(out.String(), "sudo rm "+list+" /usr/share/keyrings/lazynet.gpg"); hint != c.hint {
				t.Fatalf("retired APT hint = %v, want %v:\n%s", hint, c.hint, out)
			}
		})
	}

	// /usr/bin/lazynet that no package owns is a plain binary.
	in := &Installer{Exe: PackagePath, Output: func(string, ...string) (string, error) { return "", errors.New("no") }}
	if m := in.Method(); m != MethodBinary {
		t.Fatalf("unowned binary: %v", m)
	}
}

func TestChecksumFor(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	sums := sum + "  lazynet_0.3.0_linux_amd64.tar.gz\n" + strings.Repeat("cd", 32) + " *lazynet_0.3.0_linux_amd64.deb\nbroken line\n"
	if got := checksumFor(sums, "lazynet_0.3.0_linux_amd64.tar.gz"); got != sum {
		t.Fatalf("got %q", got)
	}
	if got := checksumFor(sums, "lazynet_0.3.0_linux_amd64.deb"); got != strings.Repeat("cd", 32) {
		t.Fatalf("binary-mode entry: %q", got)
	}
	if checksumFor(sums, "lazynet_0.3.0_linux_amd64") != "" {
		t.Fatal("prefix must not match")
	}
}
