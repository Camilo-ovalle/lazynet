// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package paloalto supports Palo Alto Networks firewalls (PAN-OS): device
// info and configuration backups in XML. lazynet works in PAN-OS operational
// mode, which transport treats as read-only: no change is ever sent.
package paloalto

import (
	"regexp"
	"strings"

	"github.com/Camilo-ovalle/lazynet/internal/platform"
)

// ID is the platform ID used in the inventory and JSON output.
const ID platform.ID = "paloalto"

// prompt matches PAN-OS operational mode: "admin@pa-fw1>" and, on HA pairs,
// "admin@pa-fw1(active)>". Cisco hostnames cannot contain '@', so a Cisco
// user-EXEC prompt ("sw1>") never matches and is still refused.
var prompt = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9._-]+(?:\([A-Za-z0-9-]+\))?>$`)

// IsPrompt reports whether p is a PAN-OS operational-mode prompt.
func IsPrompt(p string) bool { return prompt.MatchString(strings.TrimSpace(p)) }

func init() {
	platform.Register(platform.Definition{
		ID:      ID,
		Name:    "Palo Alto",
		Vendor:  "Palo Alto Networks",
		OS:      "PAN-OS",
		Aliases: []string{"panos", "pan-os"},
		Order:   40,
		Badge:   platform.Badge{Text: "PAN", Light: "#B4430B", Dark: "#FF9E64"},
		Probes: []platform.Probe{{
			Priority: 10,
			Prompt:   IsPrompt,
			Command:  "show system info",
			Match:    func(out string) bool { return strings.Contains(out, "sw-version:") },
		}},
		Session: platform.Session{
			// Paging off for this CLI session only (an operational-mode
			// command that never touches the configuration).
			Setup:              []string{"set cli pager off"},
			FailedWhenContains: []string{"Invalid syntax.", "Unknown command:", "Server error :"},
			OperationalPrompt:  IsPrompt,
		},
		New:  New,
		Demo: []platform.Demo{{Name: "pa-fw1", Group: "firewalls", Description: "Palo Alto PA-440 (emulated)"}},
	})
}

// New returns the Palo Alto driver. It implements device info and backups
// only (platform.ConfigReader); interfaces, logs and changes are reported as
// unsupported by the capability checks.
func New(s platform.Shower) platform.Driver { return &driver{s} }

type driver struct{ s platform.Shower }

func (d *driver) Info(ip string) (platform.Info, error) {
	out, err := d.s.Show("show system info")
	if err != nil {
		return platform.Info{}, err
	}
	si := ParseSystemInfo(out)
	if si.IP != "" {
		ip = si.IP // the management address the firewall reports
	}
	return platform.Info{
		Hostname: si.Hostname, Platform: ID, OS: "PAN-OS", Version: si.Version,
		Model: si.Model, Serial: si.Serial, MAC: si.MAC, IP: ip, Uptime: si.Uptime, Clock: si.Clock,
	}, nil
}

// RunningConfig returns the running configuration as PAN-OS XML, the format
// the firewall imports for a restore.
func (d *driver) RunningConfig() (string, error) { return d.s.Show("show config running") }
