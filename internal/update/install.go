// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package update

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Method is how lazynet was installed, and therefore how it is updated.
type Method int

const (
	// MethodBinary: a standalone binary (install script, tarball,
	// go install). The new binary replaces it in place, exactly as
	// scripts/install.sh would: same tarball, same checksum check, same
	// location, sudo only when that location needs it.
	MethodBinary Method = iota
	// MethodDeb: a .deb installed by hand; the new .deb is installed with dpkg.
	MethodDeb
	// MethodRPM: a .rpm installed by hand; the new .rpm is installed with rpm.
	MethodRPM
)

func (m Method) String() string {
	return [...]string{"standalone binary", ".deb package", ".rpm package"}[m]
}

// PackagePath is where the .deb and .rpm packages install the binary.
const PackagePath = "/usr/bin/lazynet"

// retiredAPTRepo identifies the APT repository that lazynet 0.1.0 offered
// and that no longer exists. A source still pointing at it makes
// `apt update` fail, so `lazynet update` explains how to remove it.
const retiredAPTRepo = "camilo-ovalle.github.io/lazynet"

// Limits for what is downloaded (the real files are a few MB).
const (
	maxPackage   = 256 << 20
	maxChecksums = 1 << 20
)

// Installer replaces the running lazynet with a newer release. Every
// download is checked against the release's checksums.txt before it is
// used.
type Installer struct {
	Client Client
	Exe    string    // the running binary, symlinks resolved
	Out    io.Writer // progress messages
	Arch   string    // GOARCH of the release to install

	// System access, replaceable in tests.
	Run      func(name string, args ...string) error // interactive: stdio attached (sudo may ask for a password)
	Output   func(name string, args ...string) (string, error)
	LookPath func(string) (string, error)
	Root     bool     // running as root: no sudo
	Sources  []string // APT source files that may still list the retired repository
}

// NewInstaller returns an installer for the running binary.
func NewInstaller(c Client, out io.Writer) (*Installer, error) {
	if runtime.GOOS != "linux" {
		return nil, errors.New("lazynet update only supports Linux for now: download the new release from " + ReleasesPage)
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	sources, _ := filepath.Glob("/etc/apt/sources.list.d/*")
	return &Installer{
		Client: c, Exe: exe, Out: out, Arch: runtime.GOARCH,
		Run: func(name string, args ...string) error {
			cmd := exec.Command(name, args...)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, out, os.Stderr
			return cmd.Run()
		},
		Output: func(name string, args ...string) (string, error) {
			b, err := exec.Command(name, args...).Output()
			return string(b), err
		},
		LookPath: exec.LookPath,
		Root:     os.Geteuid() == 0,
		Sources:  append([]string{"/etc/apt/sources.list"}, sources...),
	}, nil
}

// Method detects how the running binary was installed. Packages install
// /usr/bin/lazynet, which the install script and go install never use, and
// the package database confirms the owner.
func (in *Installer) Method() Method {
	if in.Exe != PackagePath {
		return MethodBinary
	}
	if _, err := in.Output("dpkg-query", "-S", in.Exe); err == nil {
		return MethodDeb
	}
	if _, err := in.Output("rpm", "-qf", in.Exe); err == nil {
		return MethodRPM
	}
	return MethodBinary
}

// retiredAPTSource returns the APT source file that still lists the retired
// lazynet repository, if any.
func (in *Installer) retiredAPTSource() string {
	for _, f := range in.Sources {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, l := range strings.Split(string(b), "\n") {
			l = strings.TrimSpace(l)
			if !strings.HasPrefix(l, "#") && strings.Contains(l, retiredAPTRepo) {
				return f
			}
		}
	}
	return ""
}

// Install installs rel and checks that the binary now reports it.
func (in *Installer) Install(ctx context.Context, rel Release) error {
	if in.Arch != "amd64" && in.Arch != "arm64" {
		return fmt.Errorf("no lazynet release for CPU architecture %s", in.Arch)
	}
	m := in.Method()
	fmt.Fprintf(in.Out, "Installed as a %s (%s)\n", m, in.Exe)
	var err error
	switch m {
	case MethodDeb:
		err = in.installPackage(ctx, rel, "deb", "dpkg", "-i")
	case MethodRPM:
		err = in.installPackage(ctx, rel, "rpm", "rpm", "-U")
	default:
		err = in.installBinary(ctx, rel)
	}
	if err != nil {
		return err
	}
	got, err := in.Output(in.Exe, "version")
	if err != nil {
		return fmt.Errorf("the new binary does not run: %w", err)
	}
	if !strings.HasPrefix(got, "lazynet "+rel.Version+" ") {
		return fmt.Errorf("after the update, the binary reports %q instead of lazynet %s", strings.TrimSpace(got), rel.Version)
	}
	if m == MethodDeb {
		if src := in.retiredAPTSource(); src != "" {
			fmt.Fprintf(in.Out, "\nNote: the lazynet APT repository was retired, and %s still lists it,\n"+
				"so 'apt update' reports an error for it. Remove it with:\n"+
				"  sudo rm %s /usr/share/keyrings/lazynet.gpg\n", src, src)
		}
	}
	return nil
}

func (in *Installer) installPackage(ctx context.Context, rel Release, ext string, tool ...string) error {
	dir, err := os.MkdirTemp("", "lazynet-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	pkg, err := in.fetch(ctx, rel, fmt.Sprintf("lazynet_%s_linux_%s.%s", rel.Version, in.Arch, ext), dir)
	if err != nil {
		return err
	}
	return in.privileged(append(tool, pkg)...)
}

func (in *Installer) installBinary(ctx context.Context, rel Release) error {
	dir, err := os.MkdirTemp("", "lazynet-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	archive, err := in.fetch(ctx, rel, fmt.Sprintf("lazynet_%s_linux_%s.tar.gz", rel.Version, in.Arch), dir)
	if err != nil {
		return err
	}
	bin := filepath.Join(dir, "lazynet")
	if err := extractBinary(archive, bin); err != nil {
		return err
	}
	err = replace(bin, in.Exe)
	if err == nil || !errors.Is(err, os.ErrPermission) {
		return err
	}
	// The install script puts lazynet in /usr/local/bin with sudo.
	fmt.Fprintf(in.Out, "%s is not writable by you: installing with sudo\n", filepath.Dir(in.Exe))
	return in.privileged("install", "-m", "0755", bin, in.Exe)
}

// privileged runs a command as root, through sudo unless lazynet already
// runs as root.
func (in *Installer) privileged(args ...string) error {
	if in.Root {
		return in.Run(args[0], args[1:]...)
	}
	if _, err := in.LookPath("sudo"); err != nil {
		return fmt.Errorf("%s needs root and sudo is not installed: run 'lazynet update' as root", args[0])
	}
	fmt.Fprintln(in.Out, "Running: sudo "+strings.Join(args, " "))
	return in.Run("sudo", args...)
}

// fetch downloads a release file into dir and verifies its SHA-256 against
// checksums.txt.
func (in *Installer) fetch(ctx context.Context, rel Release, name, dir string) (string, error) {
	url, ok := rel.Assets[name]
	if !ok {
		return "", fmt.Errorf("release %s has no %s", rel.Version, name)
	}
	sumsURL, ok := rel.Assets["checksums.txt"]
	if !ok {
		return "", fmt.Errorf("release %s has no checksums.txt: refusing to install an unverified download", rel.Version)
	}
	var sums strings.Builder
	if err := in.Client.download(ctx, sumsURL, &sums, maxChecksums); err != nil {
		return "", fmt.Errorf("download checksums.txt: %w", err)
	}
	want := checksumFor(sums.String(), name)
	if want == "" {
		return "", fmt.Errorf("%s is not listed in checksums.txt", name)
	}
	fmt.Fprintf(in.Out, "Downloading %s\n", name)
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	err = in.Client.download(ctx, url, io.MultiWriter(f, h), maxPackage)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("download %s: %w", name, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return "", fmt.Errorf("checksum mismatch for %s (expected %s, got %s): nothing was installed", name, want, got)
	}
	fmt.Fprintln(in.Out, "Checksum verified (SHA-256)")
	return path, nil
}

// checksumFor finds name in a sha256sum-style file.
func checksumFor(sums, name string) string {
	sc := bufio.NewScanner(strings.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == 64 {
			return strings.ToLower(f[0])
		}
	}
	return ""
}

// extractBinary copies the "lazynet" file at the root of a release tarball
// to dst.
func extractBinary(archive, dst string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(archive), err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%s does not contain the lazynet binary", filepath.Base(archive))
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", filepath.Base(archive), err)
		}
		if h.Typeflag != tar.TypeReg || strings.TrimPrefix(h.Name, "./") != "lazynet" {
			continue
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o755)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, io.LimitReader(tr, maxPackage))
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		return err
	}
}

// replace swaps dst for src atomically: the new file is written next to dst
// and renamed over it, so an interrupted update never leaves half a binary.
// The running process keeps its (old) file open until it exits.
func replace(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".lazynet-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}
