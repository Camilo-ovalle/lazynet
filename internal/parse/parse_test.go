// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package parse

import (
	"testing"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/mockssh"
)

var fixedNow = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

func dev(p string) *mockssh.Device {
	d := mockssh.NewDevice(p, "lab1")
	d.Now = func() time.Time { return fixedNow }
	return d
}

func TestInterfaces(t *testing.T) {
	c := ParseInterface(dev("catalyst").Exec("show interfaces GigabitEthernet1/0/1"))
	if c.Name != "GigabitEthernet1/0/1" || c.Status != "up" || c.Reason != "connected" || c.Description != "PC-Office-12" || c.Duplex != "full" || c.Speed != "1000Mb/s" || c.MAC == "" || c.InputRate == "" {
		t.Fatalf("catalyst: %+v", c)
	}
	down := ParseInterface(dev("catalyst").Exec("show interfaces Gi1/0/5"))
	if down.Status != "down" || down.Reason != "notconnect" {
		t.Fatalf("down: %+v", down)
	}
	n := ParseInterface(dev("nexus").Exec("show interface Ethernet1/1"))
	if n.Name != "Ethernet1/1" || n.Status != "up" || n.Mode != "trunk" || n.Description != "uplink-core" {
		t.Fatalf("nexus: %+v", n)
	}
	a := ParseInterface(dev("asa").Exec("show interface GigabitEthernet1/1"))
	if a.Name != "GigabitEthernet1/1" || a.Description != "outside" || a.Status != "up" || a.Speed != "1000 Mbps" || a.MAC != "00a3.8e4b.1c01" {
		t.Fatalf("asa: %+v", a)
	}
}

func TestSwitchportAndVlans(t *testing.T) {
	d := dev("catalyst")
	sp, ok := ParseSwitchport(d.Exec("show interfaces Gi1/0/1 switchport"))
	if !ok || sp.Mode() != "access" || sp.Dynamic() || sp.OperatingTrunk() || sp.AccessVlan != 10 || sp.VlanName != "DATA" {
		t.Fatalf("access: %+v", sp)
	}
	tr, _ := ParseSwitchport(d.Exec("show interfaces Te1/1/1 switchport"))
	if tr.Mode() != "trunk" || !tr.ConfiguredTrunk() || !tr.OperatingTrunk() {
		t.Fatalf("trunk: %+v", tr)
	}
	// A dynamic port is not a trunk: its operational mode says what it does.
	// Field regression: "dynamic auto" ports that were down were shown as
	// trunks, and their access VLAN was hidden.
	for port, want := range map[string]Switchport{
		"Gi1/0/41": {AdminMode: "dynamic auto", OperMode: "down", AccessVlan: 999, VlanName: "PARKING"},
		"Gi1/0/38": {AdminMode: "dynamic auto", OperMode: "static access", AccessVlan: 10, VlanName: "DATA"},
		"Gi1/0/39": {AdminMode: "dynamic desirable", OperMode: "trunk", AccessVlan: 10, VlanName: "DATA"},
	} {
		got, ok := ParseSwitchport(d.Exec("show interfaces " + port + " switchport"))
		if !ok || got != want || !got.Dynamic() || got.ConfiguredTrunk() || got.Mode() != want.AdminMode {
			t.Fatalf("%s: %+v, want %+v", port, got, want)
		}
		if got.OperatingTrunk() != (want.OperMode == "trunk") {
			t.Fatalf("%s: OperatingTrunk = %v", port, got.OperatingTrunk())
		}
	}
	if !(Switchport{AdminMode: "trunk", OperMode: "trunk (member of bundle Po1)"}).OperatingTrunk() {
		t.Fatal("a trunk bundle member operates as a trunk")
	}
	vl := ParseVlanBrief(d.Exec("show vlan brief"))
	if !vl[110].Usable() || vl[110].Name != "PRINTERS" || vl[1002].Usable() {
		t.Fatalf("vlans: %+v", vl)
	}
	if _, ok := vl[300]; ok {
		t.Fatal("vlan 300 should not exist")
	}
}

func TestParseLogsAllPlatforms(t *testing.T) {
	for _, p := range []string{"catalyst", "nexus", "asa"} {
		es := ParseLogs(dev(p).Exec("show logging"), fixedNow)
		if len(es) < 5 {
			t.Fatalf("%s: only %d entries", p, len(es))
		}
		for _, e := range es {
			if !e.HasTime || e.Severity < 0 || e.Facility == "" {
				t.Fatalf("%s: bad entry %+v", p, e)
			}
			if e.Time.After(fixedNow) || e.Time.Before(fixedNow.Add(-80*time.Hour)) {
				t.Fatalf("%s: time out of range %v", p, e.Time)
			}
		}
	}
}

func TestLogFormats(t *testing.T) {
	out := `Log Buffer (4096 bytes):
000123: *Nov 15 10:22:33.123: %LINK-3-UPDOWN: Interface GigabitEthernet0/1, changed state to down
000124: Nov 15 10:22:34.456 UTC: %LINEPROTO-5-UPDOWN: Line protocol on Interface GigabitEthernet0/1, changed state to down
.Dec 30 23:59:59: %SYS-5-CONFIG_I: Configured from console by vty0
1w2d: %SYS-5-RESTART: System restarted --
00:01:23: %LINK-5-CHANGED: Interface Vlan1, changed state to administratively down
%ASA-6-302013: Built inbound TCP connection
continued wrapped text that is not a log line`
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	es := ParseLogs(out, now)
	if len(es) != 6 {
		t.Fatalf("got %d entries: %+v", len(es), es)
	}
	if es[0].Time != time.Date(2025, 11, 15, 10, 22, 33, 0, time.UTC) || es[0].Severity != 3 || es[0].Mnemonic != "UPDOWN" {
		t.Fatalf("year inference / seq prefix: %+v", es[0])
	}
	if es[2].Time.Year() != 2025 {
		t.Fatalf("december entry should be previous year: %v", es[2].Time)
	}
	if es[3].HasTime || es[4].HasTime || es[5].HasTime {
		t.Fatal("uptime/unstamped entries must not have a time")
	}
	if es[5].Facility != "ASA" || es[5].Severity != 6 {
		t.Fatalf("asa mnemonic: %+v", es[5])
	}
	kept, undated := FilterLogs(es, LogFilter{From: time.Date(2025, 11, 15, 10, 22, 34, 0, time.UTC), MaxSeverity: 7})
	if len(kept) != 2 || undated != 3 {
		t.Fatalf("filter: kept %d undated %d", len(kept), undated)
	}
	kept, _ = FilterLogs(es, LogFilter{Grep: "vlan1", MaxSeverity: 7})
	if len(kept) != 1 {
		t.Fatalf("grep: %d", len(kept))
	}
	kept, _ = FilterLogs(es, LogFilter{MaxSeverity: 4})
	if len(kept) != 1 {
		t.Fatalf("severity: %d", len(kept))
	}
}

func TestParseClock(t *testing.T) {
	for _, in := range []string{"*10:22:33.123 UTC Sun Nov 15 2026", "10:22:33.123 UTC Sun Nov 15 2026\nTime source is NTP", "10:22:33.123 COT Sun Nov 15 2026"} {
		got, ok := ParseClock(in)
		if !ok || got != time.Date(2026, 11, 15, 10, 22, 33, 0, time.UTC) {
			t.Fatalf("ParseClock(%q) = %v %v", in, got, ok)
		}
	}
	if _, err := ParseUserTime("2026-11-01 08:00"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseUserTime("yesterday"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeMAC(t *testing.T) {
	for in, want := range map[string]string{"70:6D:15:A2:3B:80": "706d.15a2.3b80", "706d.15a2.3b80": "706d.15a2.3b80", "70-6d-15-a2-3b-80": "706d.15a2.3b80"} {
		if got := NormalizeMAC(in); got != want {
			t.Errorf("NormalizeMAC(%q) = %q", in, got)
		}
	}
}
