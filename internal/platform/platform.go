// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package platform is the contract between lazynet and the device families it
// supports. Each family lives in its own package under internal/platforms and
// registers a Definition from an init function; the rest of lazynet (the
// detector, the transport, the CLI and the TUI) only talks to this registry,
// so adding a vendor never means editing a switch statement elsewhere.
//
// This package names no vendor. It imports only the shared parsers (for the
// data types it exposes) and transport (for ChangeRequest), and neither of
// them imports it back.
package platform

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

// ID identifies a device family in the inventory, the detection cache and
// JSON output (e.g. "catalyst"). It is stable: never rename an ID.
type ID string

// Unknown is the zero ID: not pinned and not detected yet.
const Unknown ID = ""

// Badge is the short colored tag shown next to a device (e.g. "CAT").
// Colors are hex strings for light and dark terminals; keep about 4.5:1
// contrast on white for Light.
type Badge struct {
	Text  string
	Light string
	Dark  string
}

// Probe is one detection rule. The detector runs the probes of every
// registered platform by ascending Priority; the first Match wins. Probes
// that share a Command run it only once per detection.
type Probe struct {
	// Priority orders probes across platforms (lower runs first). Put
	// specific matches before broad ones.
	Priority int
	// Prompt, if set, must accept the login prompt for the probe to run.
	// Use it to keep a vendor's command away from other vendors' CLIs.
	Prompt func(prompt string) bool
	// Command is a read-only command (transport.CheckReadOnly must accept
	// it; a test enforces this). Keep its output short: probes run before
	// paging is disabled.
	Command string
	// Match reports whether the output identifies this platform.
	Match func(output string) bool
}

// Session describes how lazynet works in this platform's CLI.
type Session struct {
	// Setup runs once after detection, before any other command, to turn
	// paging off for this session. Each line must be a per-session setting
	// that changes nothing on the device; transport accepts these exact
	// lines in addition to "show …", and a test in internal/platforms/all
	// pins the reviewed list.
	Setup []string
	// FailedWhenContains are output markers meaning the device rejected a
	// command (e.g. "% Invalid input detected").
	FailedWhenContains []string
	// OperationalPrompt, if set, accepts a '>' prompt as this platform's
	// working mode (PAN-OS "user@host>"). Sessions opened at such a prompt
	// are read-only: transport refuses every change on them. Without it, a
	// '>' prompt is Cisco user EXEC and the login is refused.
	OperationalPrompt func(prompt string) bool
}

// Demo is an emulated device started by `lazynet --demo`. It needs a
// mockssh emulator registered under the platform ID.
type Demo struct {
	Name        string
	Group       string
	Description string
}

// Definition is everything lazynet knows about one device family.
type Definition struct {
	ID ID
	// Name is the human name ("Catalyst"); Vendor and OS are used in help
	// texts ("Cisco", "IOS / IOS-XE").
	Name   string
	Vendor string
	OS     string
	// Aliases are extra names accepted in the inventory ("panos").
	Aliases []string
	// Order sorts platforms in listings and the demo (lower first).
	Order int
	Badge Badge
	// Probes identify the platform on a fresh session.
	Probes  []Probe
	Session Session
	// New returns the driver bound to a session. It must not send commands:
	// it is also called with a nil Shower to discover capabilities.
	New  func(Shower) Driver
	Demo []Demo
}

// Supports reports whether the platform's driver implements c.
func (d Definition) Supports(c Capability) bool { return c.implementedBy(d.New(nil)) }

// Capabilities lists what the platform supports beyond device information.
func (d Definition) Capabilities() []Capability {
	var out []Capability
	for _, c := range allCapabilities {
		if d.Supports(c) {
			out = append(out, c)
		}
	}
	return out
}

// registry holds the registered definitions. Tests use their own.
type registry struct {
	mu   sync.RWMutex
	defs map[ID]Definition
	// names maps the ID and every alias (lower case) to the ID.
	names map[string]ID
}

var reg = &registry{}

// Register adds a platform. Call it from the platform package's init
// function. It panics on an invalid or duplicate definition: that is a
// programming error caught by the first test run.
func Register(d Definition) { reg.register(d) }

func (r *registry) register(d Definition) {
	if err := d.validate(); err != nil {
		panic("platform.Register: " + err.Error())
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.defs == nil {
		r.defs, r.names = map[ID]Definition{}, map[string]ID{}
	}
	for _, n := range append([]string{string(d.ID)}, d.Aliases...) {
		n = strings.ToLower(n)
		if other, dup := r.names[n]; dup {
			panic(fmt.Sprintf("platform.Register: %q is already used by %q", n, other))
		}
		r.names[n] = d.ID
	}
	r.defs[d.ID] = d
}

func (d Definition) validate() error {
	switch {
	case d.ID == Unknown || strings.ToLower(string(d.ID)) != string(d.ID) || strings.ContainsAny(string(d.ID), " \t"):
		return fmt.Errorf("invalid ID %q (use lower case, no spaces)", d.ID)
	case d.Name == "":
		return fmt.Errorf("%s: missing Name", d.ID)
	case d.Badge.Text == "":
		return fmt.Errorf("%s: missing Badge.Text", d.ID)
	case d.New == nil:
		return fmt.Errorf("%s: missing New", d.ID)
	case len(d.Probes) == 0:
		return fmt.Errorf("%s: no detection probes", d.ID)
	case len(d.Session.FailedWhenContains) == 0:
		return fmt.Errorf("%s: no Session.FailedWhenContains markers", d.ID)
	}
	for _, p := range d.Probes {
		if p.Command == "" || p.Match == nil {
			return fmt.Errorf("%s: probe needs a Command and a Match", d.ID)
		}
	}
	return nil
}

func (r *registry) lookup(id ID) (Definition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.defs[id]
	return d, ok
}

func (r *registry) parse(s string) (ID, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return Unknown, nil
	}
	r.mu.RLock()
	id, ok := r.names[s]
	r.mu.RUnlock()
	if !ok {
		return Unknown, fmt.Errorf("unknown platform %q (use %s)", s, r.namesText())
	}
	return id, nil
}

func (r *registry) all() []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Definition, 0, len(r.defs))
	for _, d := range r.defs {
		out = append(out, d)
	}
	slices.SortFunc(out, func(a, b Definition) int {
		if a.Order != b.Order {
			return a.Order - b.Order
		}
		return strings.Compare(string(a.ID), string(b.ID))
	})
	return out
}

func (r *registry) namesText() string {
	var ids []string
	for _, d := range r.all() {
		ids = append(ids, string(d.ID))
	}
	switch len(ids) {
	case 0:
		return "no platforms are registered"
	case 1:
		return ids[0]
	}
	return strings.Join(ids[:len(ids)-1], ", ") + " or " + ids[len(ids)-1]
}

// Lookup returns the definition of a registered platform.
func Lookup(id ID) (Definition, bool) { return reg.lookup(id) }

// Parse validates a user-provided platform name or alias (case-insensitive).
// The empty string is Unknown (auto-detect).
func Parse(s string) (ID, error) { return reg.parse(s) }

// All returns the registered platforms in display order.
func All() []Definition { return reg.all() }

// Names is the list of IDs for help texts: "catalyst, nexus, asa or paloalto".
func Names() string { return reg.namesText() }

// Title is the human name of a platform, or "Unknown".
func Title(id ID) string {
	if d, ok := Lookup(id); ok {
		return d.Name
	}
	return "Unknown"
}

// Supports reports whether platform id supports c. An Unknown or unregistered
// platform reports true: lazynet does not know yet, and the driver answers
// once the device is detected.
func Supports(id ID, c Capability) bool {
	d, ok := Lookup(id)
	return !ok || d.Supports(c)
}

// Setup runs the platform's session setup (paging off).
func Setup(d Definition, s Shower) error {
	for _, c := range d.Session.Setup {
		if _, err := s.Show(c); err != nil {
			return err
		}
	}
	return nil
}

// With returns the registered platforms that support c, in display order.
func With(c Capability) []Definition {
	var out []Definition
	for _, d := range All() {
		if d.Supports(c) {
			out = append(out, d)
		}
	}
	return out
}

// Describe names platforms for help texts, grouping consecutive platforms of
// the same vendor: "Cisco Catalyst, Nexus, ASA and Palo Alto".
func Describe(defs []Definition) string {
	var parts []string
	prev := ""
	for _, d := range defs {
		name := d.Name
		vendorWord, _, _ := strings.Cut(d.Vendor, " ")
		if d.Vendor != prev && vendorWord != "" && !strings.HasPrefix(name, vendorWord) {
			name = vendorWord + " " + name
		}
		prev = d.Vendor
		parts = append(parts, name)
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}
