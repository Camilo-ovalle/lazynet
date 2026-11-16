// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package nexus supports Cisco Nexus switches (NX-OS): device info,
// interfaces, logs and backups.
package nexus

import (
	"regexp"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/platforms/cisco"
)

// ID is the platform ID used in the inventory and JSON output.
const ID platform.ID = "nexus"

func init() {
	platform.Register(platform.Definition{
		ID:     ID,
		Name:   "Nexus",
		Vendor: "Cisco",
		OS:     "NX-OS",
		Order:  20,
		Badge:  platform.Badge{Text: "NXS", Light: "#6A1B9A", Dark: "#D787FF"},
		Probes: []platform.Probe{{
			Priority: 20,
			Prompt:   cisco.PrivilegedPrompt,
			Command:  cisco.VersionProbe,
			Match: func(out string) bool {
				return cisco.ContainsAny(out, "NX-OS", "Nexus Operating System")
			},
		}},
		Session: platform.Session{
			Setup:              cisco.TerminalSetup,
			FailedWhenContains: cisco.FailedWhenContains,
		},
		New:  New,
		Demo: []platform.Demo{{Name: "dc-n9k-1", Group: "datacenter", Description: "Nexus 9000 (emulated)"}},
	})
}

// New returns the Nexus driver.
func New(s platform.Shower) platform.Driver { return &driver{s} }

type driver struct{ s platform.Shower }

func (d *driver) Info(ip string) (platform.Info, error) {
	out, err := d.s.Show("show version")
	if err != nil {
		return platform.Info{}, err
	}
	v := Version(out)
	if inv, err := d.s.Show("show inventory"); err == nil {
		if sn := parse.InventorySerial(inv); sn != "" {
			v.Serial = sn
		}
	}
	if mg, err := d.s.Show("show interface mgmt0"); err == nil {
		v.MAC = parse.InterfaceMAC(mg)
		if mip := MgmtIP(mg); mip != "" && ip == "" {
			ip = mip
		}
	}
	return platform.InfoFromVersion(ID, v, ip, cisco.Clock(d.s)), nil
}

func (d *driver) Interface(name string) (platform.InterfaceDetail, error) {
	return cisco.Interface(d.s, name, "Ethernet", "show interface")
}

func (d *driver) Logs() (platform.LogResult, error) { return cisco.Logs(d.s, "show logging logfile") }

func (d *driver) RunningConfig() (string, error) { return d.s.Show("show running-config") }

var (
	nxVer    = regexp.MustCompile(`(?im)^\s*(?:NXOS|system):\s+version (\S+)`)
	nxModel  = regexp.MustCompile(`(?im)^\s*cisco (Nexus.*?) [Cc]hassis`)
	nxHost   = regexp.MustCompile(`(?im)^\s*Device name:\s*(\S+)`)
	nxUptime = regexp.MustCompile(`(?im)^Kernel uptime is (.+)$`)
	nxMgmtIP = regexp.MustCompile(`(?im)Internet Address is (\S+)`)
)

// Version parses NX-OS `show version`.
func Version(out string) parse.Version {
	return parse.Version{
		OS:       "NX-OS",
		Version:  parse.Find(nxVer, out),
		Model:    parse.Find(nxModel, out),
		Serial:   parse.BoardID(out),
		Hostname: parse.Find(nxHost, out),
		Uptime:   parse.Find(nxUptime, out),
	}
}

// MgmtIP extracts the address of `show interface mgmt0`.
func MgmtIP(out string) string { return parse.Find(nxMgmtIP, out) }
