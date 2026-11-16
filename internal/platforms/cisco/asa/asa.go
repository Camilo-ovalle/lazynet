// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package asa supports the Cisco Adaptive Security Appliance: device info,
// interfaces, logs and backups.
package asa

import (
	"regexp"
	"strings"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/platforms/cisco"
)

// ID is the platform ID used in the inventory and JSON output.
const ID platform.ID = "asa"

func init() {
	platform.Register(platform.Definition{
		ID:     ID,
		Name:   "ASA",
		Vendor: "Cisco",
		OS:     "ASA",
		Order:  30,
		Badge:  platform.Badge{Text: "ASA", Light: "#B3261E", Dark: "#FF8787"},
		Probes: []platform.Probe{{
			Priority: 10,
			Prompt:   cisco.PrivilegedPrompt,
			Command:  cisco.VersionProbe,
			Match:    func(out string) bool { return strings.Contains(out, "Adaptive Security Appliance") },
		}},
		Session: platform.Session{
			// ASA pages with "terminal pager"; "terminal length" is an error.
			Setup:              []string{"terminal pager 0"},
			FailedWhenContains: cisco.FailedWhenContains,
		},
		New:  New,
		Demo: []platform.Demo{{Name: "fw-edge", Group: "firewalls", Description: "ASA 5516 (emulated)"}},
	})
}

// New returns the ASA driver.
func New(s platform.Shower) platform.Driver { return &driver{s} }

type driver struct{ s platform.Shower }

func (d *driver) Info(ip string) (platform.Info, error) {
	out, err := d.s.Show("show version")
	if err != nil {
		return platform.Info{}, err
	}
	return platform.InfoFromVersion(ID, Version(out), ip, cisco.Clock(d.s)), nil
}

func (d *driver) Interface(name string) (platform.InterfaceDetail, error) {
	return cisco.Interface(d.s, name, "GigabitEthernet", "show interface")
}

func (d *driver) Logs() (platform.LogResult, error) { return cisco.Logs(d.s, "show logging") }

func (d *driver) RunningConfig() (string, error) { return d.s.Show("show running-config") }

var (
	asaVer      = regexp.MustCompile(`(?m)Adaptive Security Appliance Software Version (\S+)`)
	asaModel    = regexp.MustCompile(`(?m)^Hardware:\s+([^,]+),`)
	asaSerial   = regexp.MustCompile(`(?m)^Serial Number:\s*(\S+)`)
	asaUptime   = regexp.MustCompile(`(?m)^(\S+) up (.+)$`)
	asaMgmtMAC  = regexp.MustCompile(`(?m)Management\S*\s*: address is (\S+),`)
	asaFirstMAC = regexp.MustCompile(`(?m)Ext: \S+\s*: address is (\S+),`)
)

// Version parses ASA `show version`.
func Version(out string) parse.Version {
	v := parse.Version{OS: "ASA", Version: parse.Find(asaVer, out), Model: parse.Find(asaModel, out), Serial: parse.Find(asaSerial, out)}
	for _, m := range asaUptime.FindAllStringSubmatch(out, -1) {
		if m[1] != "failover" {
			v.Hostname, v.Uptime = m[1], strings.TrimSpace(m[2])
			break
		}
	}
	v.MAC = parse.Find(asaMgmtMAC, out)
	if v.MAC == "" {
		v.MAC = parse.Find(asaFirstMAC, out)
	}
	v.MAC = parse.NormalizeMAC(v.MAC)
	return v
}
