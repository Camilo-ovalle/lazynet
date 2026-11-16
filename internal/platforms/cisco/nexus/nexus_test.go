// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package nexus

import (
	"testing"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/platform/platformtest"
)

func TestVersion(t *testing.T) {
	d := platformtest.Device("nexus", "lab1")
	v := Version(d.Exec("show version"))
	if v.Version != "10.2(6)" || v.Model != "Nexus9000 C93180YC-FX" || v.Serial != "FDO24351XYZ" || v.Hostname != "lab1" || v.Uptime == "" {
		t.Fatalf("got %+v", v)
	}
	if s := parse.InventorySerial(d.Exec("show inventory")); s != "FDO24351XYZ" {
		t.Fatalf("serial %q", s)
	}
	mg := d.Exec("show interface mgmt0")
	if parse.InterfaceMAC(mg) != "2c4f.52a1.b0c8" || MgmtIP(mg) != "192.168.100.20/24" {
		t.Fatalf("mgmt0 parse failed")
	}
}

func TestDriver(t *testing.T) {
	dev, c := platformtest.Dial(t, "nexus")
	def, _ := platform.Lookup(ID)
	if err := platform.Setup(def, c); err != nil {
		t.Fatal(err)
	}
	d := New(c).(*driver)
	inf, err := d.Info("")
	if err != nil || inf.OS != "NX-OS" || inf.Serial == "" || inf.MAC == "" || inf.IP != "192.168.100.20/24" || inf.Clock.IsZero() {
		t.Fatalf("info = %+v %v", inf, err)
	}
	i, err := d.Interface("1/1") // bare numbers are Ethernet on NX-OS
	if err != nil || i.Name != "Ethernet1/1" || i.Mode != "trunk" || i.Description != "uplink-core" {
		t.Fatalf("interface = %+v %v", i, err)
	}
	if lr, err := d.Logs(); err != nil || len(lr.Entries) == 0 {
		t.Fatalf("logs: %v", err)
	}
	if _, err := d.RunningConfig(); err != nil {
		t.Fatal(err)
	}
	if def.Supports(platform.CapVlanChange) {
		t.Fatal("VLAN changes are Catalyst-only")
	}
	if bad := platformtest.ConfigCommands(dev); len(bad) > 0 {
		t.Fatalf("read-only operations sent: %v", bad)
	}
}
