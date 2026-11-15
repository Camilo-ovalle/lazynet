// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
)

// Shower runs read-only commands (implemented by *transport.Conn, which
// enforces the read-only guard).
type Shower interface {
	Show(cmd string) (string, error)
}

// ErrUnsupported is returned for features a platform does not support yet.
var ErrUnsupported = errors.New("not supported on this platform")

// Info is the "basic device information" view.
type Info struct {
	Hostname string    `json:"hostname"`
	Platform Platform  `json:"platform"`
	OS       string    `json:"os"`
	Version  string    `json:"version"`
	Model    string    `json:"model"`
	Serial   string    `json:"serial"`
	MAC      string    `json:"mac"`
	IP       string    `json:"ip"`
	Uptime   string    `json:"uptime"`
	Clock    time.Time `json:"device_clock,omitzero"`
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

// Driver is implemented by every platform.
type Driver interface {
	Platform() Platform
	Info(ip string) (Info, error)
	Interface(name string) (InterfaceDetail, error)
	Logs() (LogResult, error)
	RunningConfig() (string, error)
}

// Detect identifies the platform from `show version`. The output is filtered
// on the device ("| include Software") so it is short enough not to page
// before terminal settings are applied.
func Detect(s Shower) (Platform, error) {
	out, err := s.Show("show version | include Software")
	if err != nil {
		return Unknown, fmt.Errorf("detecting platform: %w", err)
	}
	return DetectFromVersion(out)
}

// DetectFromVersion classifies `show version` text.
func DetectFromVersion(out string) (Platform, error) {
	switch {
	case strings.Contains(out, "Adaptive Security Appliance"):
		return ASA, nil
	case strings.Contains(out, "NX-OS"), strings.Contains(out, "Nexus Operating System"):
		return Nexus, nil
	case strings.Contains(out, "IOS XE"), strings.Contains(out, "IOS-XE"), strings.Contains(out, "Cisco IOS Software"):
		return Catalyst, nil
	}
	return Unknown, fmt.Errorf("unrecognised device (show version did not match IOS, IOS-XE, NX-OS or ASA)")
}

// Setup applies per-session terminal settings (no paging, wide lines).
func Setup(p Platform, s Shower) error {
	cmds := []string{"terminal length 0", "terminal width 511"}
	if p == ASA {
		cmds = []string{"terminal pager 0"}
	}
	for _, c := range cmds {
		if _, err := s.Show(c); err != nil {
			return err
		}
	}
	return nil
}

// New returns the driver for p.
func New(p Platform, s Shower) (Driver, error) {
	switch p {
	case Catalyst:
		return &catalyst{s}, nil
	case Nexus:
		return &nexus{s}, nil
	case ASA:
		return &asa{s}, nil
	}
	return nil, fmt.Errorf("no driver for platform %q", p)
}

func clock(s Shower) time.Time {
	out, err := s.Show("show clock")
	if err != nil {
		return time.Time{}
	}
	t, _ := parse.ParseClock(out)
	return t
}

func logs(s Shower, cmd string) (LogResult, error) {
	now := clock(s)
	out, err := s.Show(cmd)
	if err != nil {
		return LogResult{}, err
	}
	return LogResult{Entries: parse.ParseLogs(out, now), DeviceClock: now, Command: cmd}, nil
}

func iface(s Shower, p Platform, name, cmd string) (InterfaceDetail, error) {
	n, err := NormalizeInterface(name, p)
	if err != nil {
		return InterfaceDetail{}, err
	}
	out, err := s.Show(cmd + " " + n)
	if err != nil {
		return InterfaceDetail{}, err
	}
	return InterfaceDetail{Interface: parse.ParseInterface(out), Raw: out}, nil
}

// --- Catalyst (IOS / IOS-XE) ---

type catalyst struct{ s Shower }

func (c *catalyst) Platform() Platform { return Catalyst }

func (c *catalyst) Info(ip string) (Info, error) {
	out, err := c.s.Show("show version")
	if err != nil {
		return Info{}, err
	}
	v := parse.IOSVersion(out)
	if v.Serial == "" {
		if inv, err := c.s.Show("show inventory"); err == nil {
			v.Serial = parse.InventorySerial(inv)
		}
	}
	return info(Catalyst, v, ip, clock(c.s)), nil
}

func (c *catalyst) Interface(name string) (InterfaceDetail, error) {
	d, err := iface(c.s, Catalyst, name, "show interfaces")
	if err != nil {
		return d, err
	}
	if out, err := c.s.Show("show interfaces " + d.Name + " switchport"); err == nil {
		if sp, ok := parse.ParseSwitchport(out); ok {
			d.Switchport = &sp
			d.AccessVlan = sp.AccessVlan
			d.Mode = "access"
			if sp.IsTrunk() {
				d.Mode = "trunk"
			}
		}
	} else if d.Mode == "" {
		d.Mode = "routed"
	}
	return d, nil
}

func (c *catalyst) Logs() (LogResult, error)       { return logs(c.s, "show logging") }
func (c *catalyst) RunningConfig() (string, error) { return c.s.Show("show running-config") }

// --- Nexus (NX-OS) ---

type nexus struct{ s Shower }

func (n *nexus) Platform() Platform { return Nexus }

func (n *nexus) Info(ip string) (Info, error) {
	out, err := n.s.Show("show version")
	if err != nil {
		return Info{}, err
	}
	v := parse.NXOSVersion(out)
	if inv, err := n.s.Show("show inventory"); err == nil {
		if sn := parse.InventorySerial(inv); sn != "" {
			v.Serial = sn
		}
	}
	if mg, err := n.s.Show("show interface mgmt0"); err == nil {
		v.MAC = parse.InterfaceMAC(mg)
		if mip := parse.NXOSMgmtIP(mg); mip != "" && ip == "" {
			ip = mip
		}
	}
	return info(Nexus, v, ip, clock(n.s)), nil
}

func (n *nexus) Interface(name string) (InterfaceDetail, error) {
	return iface(n.s, Nexus, name, "show interface")
}
func (n *nexus) Logs() (LogResult, error)       { return logs(n.s, "show logging logfile") }
func (n *nexus) RunningConfig() (string, error) { return n.s.Show("show running-config") }

// --- ASA ---

type asa struct{ s Shower }

func (a *asa) Platform() Platform { return ASA }

func (a *asa) Info(ip string) (Info, error) {
	out, err := a.s.Show("show version")
	if err != nil {
		return Info{}, err
	}
	return info(ASA, parse.ASAVersion(out), ip, clock(a.s)), nil
}

func (a *asa) Interface(name string) (InterfaceDetail, error) {
	return iface(a.s, ASA, name, "show interface")
}
func (a *asa) Logs() (LogResult, error)       { return logs(a.s, "show logging") }
func (a *asa) RunningConfig() (string, error) { return a.s.Show("show running-config") }

func info(p Platform, v parse.Version, ip string, clk time.Time) Info {
	return Info{
		Hostname: v.Hostname, Platform: p, OS: v.OS, Version: v.Version, Model: v.Model,
		Serial: v.Serial, MAC: v.MAC, IP: ip, Uptime: v.Uptime, Clock: clk,
	}
}
