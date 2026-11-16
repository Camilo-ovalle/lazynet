// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package asa

import (
	"testing"

	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/platform/platformtest"
)

func TestVersion(t *testing.T) {
	v := Version(platformtest.Device("asa", "lab1").Exec("show version"))
	if v.Version != "9.16(4)19" || v.Model != "ASA5516" || v.Serial != "JAD21430ABC" || v.Hostname != "lab1" || v.Uptime != "41 days 7 hours" || v.MAC != "00a3.8e4b.1c00" {
		t.Fatalf("got %+v", v)
	}
}

func TestDriver(t *testing.T) {
	dev, c := platformtest.Dial(t, "asa")
	def, _ := platform.Lookup(ID)
	if err := platform.Setup(def, c); err != nil { // terminal pager 0
		t.Fatal(err)
	}
	d := New(c).(*driver)
	inf, err := d.Info("10.0.0.1")
	if err != nil || inf.OS != "ASA" || inf.Serial == "" || inf.MAC == "" || inf.Uptime == "" || inf.Clock.IsZero() {
		t.Fatalf("info = %+v %v", inf, err)
	}
	i, err := d.Interface("gi1/1")
	if err != nil || i.Name != "GigabitEthernet1/1" || i.Description != "outside" {
		t.Fatalf("interface = %+v %v", i, err)
	}
	if lr, err := d.Logs(); err != nil || len(lr.Entries) == 0 {
		t.Fatalf("logs: %v", err)
	}
	if _, err := d.RunningConfig(); err != nil {
		t.Fatal(err)
	}
	if bad := platformtest.ConfigCommands(dev); len(bad) > 0 {
		t.Fatalf("read-only operations sent: %v", bad)
	}
}
