// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package mockssh is a fake network-device SSH server used by tests and demo
// mode. It emulates prompts, privilege/config modes, output filters and a
// small amount of state (switchport VLANs) so the full VLAN-change workflow
// can be exercised without touching a real device.
//
// Each platform's CLI is an emulator in its own file (catalyst.go,
// paloalto.go, ...) registered under the platform ID, with its canned
// outputs in fixtures/<platform>/. Cisco-style CLIs share the engine in
// ios.go. This package does not import the platform packages, so transport
// and platform tests can use it freely.
package mockssh

import (
	"embed"
	"slices"
	"strings"
	"sync"
	"time"
)

//go:embed fixtures
var fixtures embed.FS

// Port is an emulated interface.
type Port struct {
	Name  string // canonical, e.g. GigabitEthernet1/0/1
	Short string // e.g. Gi1/0/1
	Up    bool
	Desc  string
	Trunk bool
	Vlan  int
	MAC   string
	// Mode is the administrative switchport mode of a non-trunk port on the
	// IOS emulators: "" for static access, or "dynamic auto" (the Catalyst
	// default) / "dynamic desirable", which negotiate with DTP.
	Mode string
	// PeerTrunks makes a dynamic port negotiate a trunk while it is up, as
	// when its neighbour is a switch or an AP set to trunk.
	PeerTrunks bool
}

// operTrunk reports whether the port is operating as a trunk.
func (p *Port) operTrunk() bool { return p.Trunk || (p.Up && p.Mode != "" && p.PeerTrunks) }

// Device is the emulated device state.
type Device struct {
	Hostname     string
	Platform     string // a registered emulator, e.g. catalyst
	Unprivileged bool   // log in at '>' instead of '#' (Cisco-style CLIs)
	Now          func() time.Time

	emu      emulator
	mu       sync.Mutex
	ports    []*Port
	vlans    map[int]string
	commands []string
	saved    bool
	static   map[string]string // fixtures: command -> output
}

// emulator is the CLI of one platform. Methods run with Device.mu held.
type emulator interface {
	// setup builds the device's default state (ports, VLANs, ...).
	setup(d *Device)
	prompt(s *session) string
	// exec runs one non-empty line and reports whether to close the session.
	exec(s *session, line string) (out string, closeSession bool)
}

var emulators = map[string]emulator{}

func register(platform string, e emulator) {
	if _, dup := emulators[platform]; dup {
		panic("mockssh: duplicate emulator " + platform)
	}
	emulators[platform] = e
}

// Platforms lists the platforms that have an emulator.
func Platforms() []string {
	out := make([]string, 0, len(emulators))
	for p := range emulators {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

// Emulates reports whether platform has an emulator.
func Emulates(platform string) bool {
	_, ok := emulators[platform]
	return ok
}

// NewDevice builds a device with realistic default state for the platform.
// Fixtures in fixtures/<platform>/ answer the command named after the file
// ("show_version.txt" answers "show version"). An unknown platform gets a
// bare Cisco-style CLI.
func NewDevice(platform, hostname string) *Device {
	d := &Device{Hostname: hostname, Platform: platform, Now: time.Now, vlans: map[int]string{}, static: map[string]string{}}
	entries, _ := fixtures.ReadDir("fixtures/" + platform)
	for _, e := range entries {
		b, _ := fixtures.ReadFile("fixtures/" + platform + "/" + e.Name())
		cmd := strings.ReplaceAll(strings.TrimSuffix(e.Name(), ".txt"), "_", " ")
		d.static[cmd] = string(b)
	}
	d.emu = emulators[platform]
	if d.emu == nil {
		d.emu = &ios{}
	}
	d.emu.setup(d)
	return d
}

// Commands returns every command line received so far (for assertions).
func (d *Device) Commands() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.commands...)
}

// Saved reports whether the configuration was saved (write memory).
func (d *Device) Saved() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.saved
}

// PortVlan returns the access VLAN of a port (canonical name).
func (d *Device) PortVlan(name string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	if p := d.port(name); p != nil {
		return p.Vlan
	}
	return -1
}

func (d *Device) port(name string) *Port {
	n := strings.ToLower(name)
	for _, p := range d.ports {
		if strings.ToLower(p.Name) == n || strings.ToLower(p.Short) == n {
			return p
		}
	}
	return nil
}

// render fills fixture placeholders.
func (d *Device) render(s string, now time.Time) string {
	s = strings.ReplaceAll(s, "{{HOST}}", d.Hostname)
	s = strings.ReplaceAll(s, "{{NOW}}", now.UTC().Format("Mon Jan  2 15:04:05 2006"))
	return strings.TrimRight(s, "\n")
}

// Exec runs one command in a fresh privileged session and returns its output
// (used by parser tests to reuse the emulator's fixtures).
func (d *Device) Exec(cmd string) string {
	s := &session{d: d, mode: modePriv}
	out, _ := s.handle(cmd)
	return out
}

type mode int

const (
	modeExec mode = iota
	modePriv
	modeConfig
	modeConfigIf
)

// session is per-connection CLI state.
type session struct {
	d     *Device
	mode  mode
	iface *Port
	user  string // SSH login (some prompts show it)
}

func (s *session) prompt() string {
	s.d.mu.Lock()
	defer s.d.mu.Unlock()
	return s.d.emu.prompt(s)
}

// handle executes one line and returns output and whether to close.
func (s *session) handle(line string) (string, bool) {
	line = strings.TrimSpace(line)
	d := s.d
	d.mu.Lock()
	defer d.mu.Unlock()
	if line == "" {
		return "", false
	}
	d.commands = append(d.commands, line)
	return d.emu.exec(s, line)
}
