// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/transport"
)

// Shower runs read-only commands. *transport.Conn implements it and enforces
// the read-only guard, so a driver cannot change anything through it.
type Shower interface {
	Show(cmd string) (string, error)
}

// Applier can also send confirmed change requests (*transport.Conn).
type Applier interface {
	Shower
	Apply(*transport.ChangeRequest) (string, error)
}

// Driver is what every platform implements: device information. Everything
// else is an optional capability (see Capability): implement the matching
// interface and the CLI and the TUI offer the feature automatically.
type Driver interface {
	Info(ip string) (Info, error)
}

// InterfaceReader reads the state of one interface (CapInterfaces).
type InterfaceReader interface {
	Interface(name string) (InterfaceDetail, error)
}

// LogReader reads the device log buffer (CapLogs).
type LogReader interface {
	Logs() (LogResult, error)
}

// ConfigReader returns the running configuration for backups (CapBackup).
type ConfigReader interface {
	RunningConfig() (string, error)
}

// VlanChanger changes the access VLAN of a port (CapVlanChange). This is the
// only configuration change lazynet makes; read AGENTS.md (safety rules)
// before implementing it for a new platform.
type VlanChanger interface {
	// PlanVlanChange validates the change with show commands only and
	// returns the change request to show the user. It changes nothing.
	PlanVlanChange(port string, vlan int) (*VlanPlan, error)
	// ApplyVlanChange sends a plan the user confirmed and verifies it by
	// reading the port back.
	ApplyVlanChange(a Applier, plan *VlanPlan) (string, error)
	// SaveRequest persists running-config (asked separately after a change).
	SaveRequest() *transport.ChangeRequest
}

// Capability is an optional feature of a driver.
type Capability int

const (
	CapInterfaces Capability = iota
	CapLogs
	CapBackup
	CapVlanChange
)

var allCapabilities = []Capability{CapInterfaces, CapLogs, CapBackup, CapVlanChange}

func (c Capability) String() string {
	switch c {
	case CapInterfaces:
		return "interface details"
	case CapLogs:
		return "logs"
	case CapBackup:
		return "configuration backups"
	case CapVlanChange:
		return "VLAN changes"
	}
	return fmt.Sprintf("capability(%d)", int(c))
}

func (c Capability) implementedBy(d Driver) bool {
	var ok bool
	switch c {
	case CapInterfaces:
		_, ok = d.(InterfaceReader)
	case CapLogs:
		_, ok = d.(LogReader)
	case CapBackup:
		_, ok = d.(ConfigReader)
	case CapVlanChange:
		_, ok = d.(VlanChanger)
	}
	return ok
}

// ErrUnsupported is returned for features a platform does not support yet.
var ErrUnsupported = errors.New("not supported")

// Unsupported explains that platform id lacks capability c and lists what it
// does support. It wraps ErrUnsupported.
func Unsupported(id ID, c Capability) error {
	d, ok := Lookup(id)
	if !ok {
		return fmt.Errorf("%s %w on this platform", c, ErrUnsupported)
	}
	avail := []string{"device info"}
	for _, x := range d.Capabilities() {
		avail = append(avail, x.String())
	}
	return fmt.Errorf("%s: %s %w yet (available: %s)", d.Name, c, ErrUnsupported, strings.Join(avail, ", "))
}

// Info is the "basic device information" view.
type Info struct {
	Hostname string    `json:"hostname"`
	Platform ID        `json:"platform"`
	OS       string    `json:"os"`
	Version  string    `json:"version"`
	Model    string    `json:"model"`
	Serial   string    `json:"serial"`
	MAC      string    `json:"mac"`
	IP       string    `json:"ip"`
	Uptime   string    `json:"uptime"`
	Clock    time.Time `json:"device_clock,omitzero"`
}

// InfoFromVersion builds Info from parsed `show version` data.
func InfoFromVersion(p ID, v parse.Version, ip string, clock time.Time) Info {
	return Info{
		Hostname: v.Hostname, Platform: p, OS: v.OS, Version: v.Version, Model: v.Model,
		Serial: v.Serial, MAC: v.MAC, IP: ip, Uptime: v.Uptime, Clock: clock,
	}
}

// InterfaceDetail combines parsed interface data with the raw output.
type InterfaceDetail struct {
	parse.Interface
	Switchport *parse.Switchport `json:"switchport,omitempty"`
	Raw        string            `json:"raw"`
}

// LogResult is the parsed log buffer plus the device clock.
type LogResult struct {
	Entries     []parse.LogEntry
	DeviceClock time.Time
	Command     string
}

// VlanPlan is a validated, ready-to-confirm VLAN change.
type VlanPlan struct {
	Port string `json:"port"`
	// PortMode describes the switchport mode, e.g. "dynamic auto,
	// operational: down".
	PortMode        string `json:"port_mode,omitempty"`
	CurrentVlan     int    `json:"current_vlan"`
	CurrentVlanName string `json:"current_vlan_name"`
	TargetVlan      int    `json:"target_vlan"`
	TargetVlanName  string `json:"target_vlan_name"`
	// Warnings must be shown with the confirmation, e.g. that a dynamic
	// port keeps negotiating with DTP.
	Warnings []string `json:"warnings,omitempty"`
	// NoOp is true when the port is already in the target VLAN.
	NoOp bool `json:"no_op"`
	// Change is the request to confirm (nil when NoOp).
	Change *transport.ChangeRequest `json:"-"`
	// Save persists the change; it is offered separately after a
	// successful, verified apply.
	Save *transport.ChangeRequest `json:"-"`
}
