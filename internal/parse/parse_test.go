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

func TestIOSXEVersion(t *testing.T) {
	v := IOSVersion(dev("catalyst").Exec("show version"))
	want := Version{Hostname: "lab1", OS: "IOS-XE", Version: "17.09.04a", Model: "C9300-48P", Serial: "FCW2318L0AB", MAC: "706d.15a2.3b80", Uptime: "12 weeks, 3 days, 4 hours, 51 minutes"}
	if v != want {
		t.Fatalf("got  %+v\nwant %+v", v, want)
	}
}

const classicIOS = `Cisco IOS Software, C2960X Software (C2960X-UNIVERSALK9-M), Version 15.2(7)E8, RELEASE SOFTWARE (fc1)
Technical Support: http://www.cisco.com/techsupport
ROM: Bootstrap program is C2960X boot loader
access-sw2 uptime is 1 year, 2 weeks, 3 days, 5 hours, 1 minute
System image file is "flash:c2960x-universalk9-mz.152-7.E8.bin"
cisco WS-C2960X-48FPD-L (APM86XXX) processor (revision D0) with 524288K bytes of memory.
Processor board ID FOC1951X2AB
Base ethernet MAC Address       : 70:6D:15:11:22:00
Model number                    : WS-C2960X-48FPD-L
System serial number            : FOC1951X2AB
`

func TestClassicIOSVersion(t *testing.T) {
	v := IOSVersion(classicIOS)
	if v.OS != "IOS" || v.Version != "15.2(7)E8" || v.Model != "WS-C2960X-48FPD-L" || v.Serial != "FOC1951X2AB" || v.MAC != "706d.1511.2200" || v.Hostname != "access-sw2" {
		t.Fatalf("got %+v", v)
	}
}

func TestNXOSVersion(t *testing.T) {
	d := dev("nexus")
	v := NXOSVersion(d.Exec("show version"))
	if v.Version != "10.2(6)" || v.Model != "Nexus9000 C93180YC-FX" || v.Serial != "FDO24351XYZ" || v.Hostname != "lab1" || v.Uptime == "" {
		t.Fatalf("got %+v", v)
	}
	if s := InventorySerial(d.Exec("show inventory")); s != "FDO24351XYZ" {
		t.Fatalf("serial %q", s)
	}
	mg := d.Exec("show interface mgmt0")
	if InterfaceMAC(mg) != "2c4f.52a1.b0c8" || NXOSMgmtIP(mg) != "192.168.100.20/24" {
		t.Fatalf("mgmt0 parse failed")
	}
}

func TestASAVersion(t *testing.T) {
	v := ASAVersion(dev("asa").Exec("show version"))
	if v.Version != "9.16(4)19" || v.Model != "ASA5516" || v.Serial != "JAD21430ABC" || v.Hostname != "lab1" || v.Uptime != "41 days 7 hours" || v.MAC != "00a3.8e4b.1c00" {
		t.Fatalf("got %+v", v)
	}
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
	if !ok || sp.IsTrunk() || sp.AccessVlan != 10 || sp.VlanName != "DATA" {
		t.Fatalf("access: %+v", sp)
	}
	tr, _ := ParseSwitchport(d.Exec("show interfaces Te1/1/1 switchport"))
	if !tr.IsTrunk() {
		t.Fatalf("trunk: %+v", tr)
	}
	if (Switchport{AdminMode: "dynamic auto", OperMode: "static access"}).IsTrunk() == false {
		t.Fatal("dynamic modes must count as trunk-capable")
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

func TestPANOSSystemInfo(t *testing.T) {
	si := PANOSSystemInfo(dev("paloalto").Exec("show system info"))
	if si.Hostname != "lab1" || si.Model != "PA-440" || si.Serial != "024101001234" || si.Version != "11.1.4-h7" ||
		si.MAC != "00:50:56:a1:b2:c3" || si.IP != "10.10.0.2" || si.Uptime != "41 days, 6:12:40" {
		t.Fatalf("got %+v", si)
	}
	if !si.Clock.Equal(fixedNow.UTC().Truncate(time.Second)) {
		t.Fatalf("clock = %v, want %v", si.Clock, fixedNow.UTC())
	}
	// "unknown" values are treated as empty.
	if PANOSSystemInfo("serial: unknown\nip-address: unknown\n").Serial != "" {
		t.Fatal("unknown serial should be empty")
	}
}
