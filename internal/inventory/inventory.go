// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package inventory loads the user's device inventory (inventory.yml).
package inventory

import (
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Device is one network device the user can connect to.
type Device struct {
	Name     string `yaml:"name"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port,omitempty"`
	Group    string `yaml:"group,omitempty"`
	Username string `yaml:"username,omitempty"`
	// Platform optionally pins the platform (catalyst|nexus|asa) and skips
	// auto-detection. Leave empty to auto-detect.
	Platform    string `yaml:"platform,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// Addr returns host:port for dialing.
func (d Device) Addr() string { return net.JoinHostPort(d.Host, strconv.Itoa(d.SSHPort())) }

// SSHPort returns the configured port or 22.
func (d Device) SSHPort() int {
	if d.Port == 0 {
		return 22
	}
	return d.Port
}

// Label is the friendliest identifier for display and file names.
func (d Device) Label() string {
	if d.Name != "" {
		return d.Name
	}
	return d.Host
}

// Inventory is the parsed inventory file.
type Inventory struct {
	Defaults struct {
		Username string `yaml:"username,omitempty"`
		Port     int    `yaml:"port,omitempty"`
	} `yaml:"defaults,omitempty"`
	Devices []Device `yaml:"devices"`

	path string
}

// Load reads the inventory. A missing file yields an empty inventory.
func Load(path string) (*Inventory, error) {
	inv := &Inventory{path: path}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return inv, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(b, inv); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i := range inv.Devices {
		d := &inv.Devices[i]
		if d.Username == "" {
			d.Username = inv.Defaults.Username
		}
		if d.Port == 0 {
			d.Port = inv.Defaults.Port
		}
		d.Platform = strings.ToLower(d.Platform)
	}
	return inv, nil
}

// Path returns where the inventory was loaded from.
func (inv *Inventory) Path() string { return inv.path }

// Resolve turns a CLI target (inventory name, host, or host:port) into a Device.
// Unknown targets become ad-hoc devices.
func (inv *Inventory) Resolve(target string) Device {
	for _, d := range inv.Devices {
		if strings.EqualFold(d.Name, target) || d.Addr() == target {
			return d
		}
	}
	for _, d := range inv.Devices {
		if d.Host == target {
			return d
		}
	}
	d := Device{Host: target, Username: inv.Defaults.Username, Port: inv.Defaults.Port}
	if h, p, err := net.SplitHostPort(target); err == nil {
		if n, err := strconv.Atoi(p); err == nil {
			d.Host, d.Port = h, n
		}
	}
	return d
}

// Group returns the devices in a group (case-insensitive).
func (inv *Inventory) Group(name string) []Device {
	var out []Device
	for _, d := range inv.Devices {
		if strings.EqualFold(d.Group, name) {
			out = append(out, d)
		}
	}
	return out
}

// Groups returns the sorted list of group names ("" = ungrouped, last).
func (inv *Inventory) Groups() []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range inv.Devices {
		if !seen[d.Group] {
			seen[d.Group] = true
			out = append(out, d.Group)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i] == "" || out[j] == "" {
			return out[j] == ""
		}
		return out[i] < out[j]
	})
	return out
}

// Validate reports structural problems (missing host, duplicate names, bad platform).
func (inv *Inventory) Validate() []error {
	var errs []error
	names := map[string]bool{}
	for i, d := range inv.Devices {
		where := fmt.Sprintf("device #%d (%s)", i+1, d.Label())
		if d.Host == "" {
			errs = append(errs, fmt.Errorf("%s: missing host", where))
		}
		if d.Name != "" {
			k := strings.ToLower(d.Name)
			if names[k] {
				errs = append(errs, fmt.Errorf("%s: duplicate name", where))
			}
			names[k] = true
		}
		switch d.Platform {
		case "", "catalyst", "nexus", "asa":
		default:
			errs = append(errs, fmt.Errorf("%s: unknown platform %q (use catalyst, nexus or asa)", where, d.Platform))
		}
	}
	return errs
}

// Add appends a device to the inventory file, preserving existing comments
// and formatting by editing the YAML node tree instead of re-marshalling.
func Add(path string, d Device) error {
	var root yaml.Node
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist) || (err == nil && len(strings.TrimSpace(string(b))) == 0):
		root = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	case err != nil:
		return err
	default:
		if err := yaml.Unmarshal(b, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}
	m := root.Content[0]
	if m.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: top level must be a mapping", path)
	}
	var devices *yaml.Node
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == "devices" {
			devices = m.Content[i+1]
		}
	}
	if devices == nil {
		devices = &yaml.Node{Kind: yaml.SequenceNode}
		m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "devices"}, devices)
	}
	if devices.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s: 'devices' must be a list", path)
	}
	var dn yaml.Node
	if err := dn.Encode(d); err != nil {
		return err
	}
	devices.Content = append(devices.Content, &dn)
	out, err := yaml.Marshal(&root)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

// Example is written by `lazynet inventory init`.
const Example = `# lazynet inventory
# Each device needs at least a host. "platform" is optional: lazynet detects
# Catalyst (IOS / IOS-XE), Nexus (NX-OS) and ASA automatically.
defaults:
  username: admin
  # port: 22

devices:
  - name: core-sw1
    host: 192.168.100.100
    group: core
    description: Core switch, rack A1
  - name: access-sw-3f
    host: 192.168.100.110
    group: access
  - name: dc-n9k-1
    host: 192.168.100.20
    group: datacenter
  - name: fw-edge
    host: 192.168.100.1
    group: firewalls
    username: fwadmin
`
