// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package catalyst supports Cisco Catalyst switches (classic IOS and IOS-XE):
// device info, interfaces, logs, backups and the guarded access-VLAN change.
package catalyst

import (
	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/platforms/cisco"
)

// ID is the platform ID used in the inventory and JSON output.
const ID platform.ID = "catalyst"

// bare is the interface type of a bare number ("1/0/1").
const bare = "GigabitEthernet"

func init() {
	platform.Register(platform.Definition{
		ID:     ID,
		Name:   "Catalyst",
		Vendor: "Cisco",
		OS:     "IOS / IOS-XE",
		Order:  10,
		Badge:  platform.Badge{Text: "CAT", Light: "#1F5FAD", Dark: "#5FAFFF"},
		Probes: []platform.Probe{{
			// Last among the Cisco probes: "Cisco IOS Software" is the
			// broadest match.
			Priority: 30,
			Prompt:   cisco.PrivilegedPrompt,
			Command:  cisco.VersionProbe,
			Match: func(out string) bool {
				return cisco.ContainsAny(out, "IOS XE", "IOS-XE", "Cisco IOS Software")
			},
		}},
		Session: platform.Session{
			Setup:              cisco.TerminalSetup,
			FailedWhenContains: cisco.FailedWhenContains,
		},
		New: New,
		Demo: []platform.Demo{
			{Name: "core-sw1", Group: "core", Description: "Catalyst 9300 core switch (emulated)"},
			{Name: "access-sw2", Group: "access", Description: "Catalyst access switch (emulated)"},
		},
	})
}

// New returns the Catalyst driver.
func New(s platform.Shower) platform.Driver { return &driver{s} }

type driver struct{ s platform.Shower }

func (d *driver) Info(ip string) (platform.Info, error) {
	out, err := d.s.Show("show version")
	if err != nil {
		return platform.Info{}, err
	}
	v := Version(out)
	if v.Serial == "" {
		if inv, err := d.s.Show("show inventory"); err == nil {
			v.Serial = parse.InventorySerial(inv)
		}
	}
	return platform.InfoFromVersion(ID, v, ip, cisco.Clock(d.s)), nil
}

func (d *driver) Interface(name string) (platform.InterfaceDetail, error) {
	det, err := cisco.Interface(d.s, name, bare, "show interfaces")
	if err != nil {
		return det, err
	}
	if out, err := d.s.Show("show interfaces " + det.Name + " switchport"); err == nil {
		if sp, ok := parse.ParseSwitchport(out); ok {
			// The configured mode, as the device reports it: a dynamic
			// port is shown as such, never as a trunk it isn't.
			det.Switchport = &sp
			det.AccessVlan = sp.AccessVlan
			det.Mode = sp.Mode()
		}
	} else if det.Mode == "" {
		det.Mode = "routed"
	}
	return det, nil
}

func (d *driver) Logs() (platform.LogResult, error) { return cisco.Logs(d.s, "show logging") }

func (d *driver) RunningConfig() (string, error) { return d.s.Show("show running-config") }
