// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package catalyst

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/render"
	"github.com/charmbracelet/x/ansi"
)

func switchportFixture(t *testing.T, name string) parse.Switchport {
	t.Helper()
	b, err := os.ReadFile("testdata/switchport_" + name + ".txt")
	if err != nil {
		t.Fatal(err)
	}
	sp, ok := parse.ParseSwitchport(string(b))
	if !ok {
		t.Fatalf("%s: not parsed as a switchport", name)
	}
	return sp
}

// The rule for changing an access VLAN, on IOS XE 17.12 output. Field
// regression: "dynamic auto" ports (the C9200L default) were refused as
// trunks, so most access ports couldn't be changed.
func TestCheckAccessPort(t *testing.T) {
	cases := []struct {
		fixture string
		err     error
		warn    bool
		mode    string
	}{
		{"dynamic_auto_down", nil, true, "dynamic auto, operational: down"},
		{"dynamic_auto_access", nil, true, "dynamic auto, operational: static access"},
		{"dynamic_desirable_trunk", ErrTrunkPort, false, "dynamic desirable, operational: trunk"},
		{"access_bundle_member", ErrNotAccessPort, false, "static access, operational: static access (member of bundle po2)"},
		{"private_vlan_host", ErrNotAccessPort, false, "private-vlan host, operational: down"},
	}
	for _, c := range cases {
		sp := switchportFixture(t, c.fixture)
		if sp.AccessVlan != 1200 || sp.VlanName != "USERS-2F" {
			t.Fatalf("%s: access VLAN %d %q", c.fixture, sp.AccessVlan, sp.VlanName)
		}
		warnings, err := checkAccessPort("GigabitEthernet1/0/25", sp)
		if !errors.Is(err, c.err) {
			t.Errorf("%s: err = %v, want %v", c.fixture, err, c.err)
		}
		if (len(warnings) > 0) != c.warn {
			t.Errorf("%s: warnings = %q", c.fixture, warnings)
		}
		if got := portMode(sp); got != c.mode {
			t.Errorf("%s: portMode = %q, want %q", c.fixture, got, c.mode)
		}
	}
	for _, sp := range []parse.Switchport{
		{AdminMode: "static access", OperMode: "static access"},
		{AdminMode: "static access", OperMode: "down"},
	} {
		if w, err := checkAccessPort("Gi1/0/1", sp); err != nil || len(w) != 0 {
			t.Errorf("%+v: %v %q", sp, err, w)
		}
	}
	for _, sp := range []parse.Switchport{
		{AdminMode: "trunk", OperMode: "trunk"},
		{AdminMode: "trunk", OperMode: "down"},
		{AdminMode: "trunk", OperMode: "trunk (member of bundle Po1)"},
	} {
		if _, err := checkAccessPort("Te1/1/1", sp); !errors.Is(err, ErrTrunkPort) {
			t.Errorf("%+v: %v", sp, err)
		}
	}
	for _, sp := range []parse.Switchport{
		{AdminMode: "dot1q-tunnel", OperMode: "down"},
		{AdminMode: "dynamic auto", OperMode: "something new"},
		{AdminMode: "dynamic auto", OperMode: ""},
	} {
		if _, err := checkAccessPort("Gi1/0/2", sp); !errors.Is(err, ErrNotAccessPort) {
			t.Errorf("%+v: %v", sp, err)
		}
	}
}

// The Interface tab shows what the switch reports: the configured mode, the
// operational mode when it differs, and the access VLAN of a dynamic port.
func TestInterfaceDynamicPorts(t *testing.T) {
	d, _, config := driverFor(t)
	cases := []struct {
		port, mode, oper string
		vlan             int
		shown            []string
	}{
		{"1/0/41", "dynamic auto", "down", 999, []string{"dynamic auto · operational: down", "Access VLAN  999 PARKING"}},
		{"1/0/38", "dynamic auto", "static access", 10, []string{"dynamic auto · operational: static access", "Access VLAN  10 DATA"}},
		{"1/0/39", "dynamic desirable", "trunk", 10, []string{"dynamic desirable · operational: trunk", "10 DATA (not in use: operating as a trunk)"}},
		{"1/0/1", "access", "static access", 10, []string{"Mode         access\n", "Access VLAN  10 DATA"}},
		{"1/0/5", "access", "down", 10, []string{"access · operational: down"}},
		{"te1/1/1", "trunk", "trunk", 1, []string{"Mode         trunk\n", "Access VLAN  —\n"}},
	}
	for _, c := range cases {
		i, err := d.Interface(c.port)
		if err != nil {
			t.Fatal(err)
		}
		if i.Mode != c.mode || i.AccessVlan != c.vlan || i.Switchport == nil || i.Switchport.OperMode != c.oper {
			t.Fatalf("%s: mode %q vlan %d switchport %+v", c.port, i.Mode, i.AccessVlan, i.Switchport)
		}
		out := ansi.Strip(render.Interface(i))
		for _, s := range c.shown {
			if !strings.Contains(out, s) {
				t.Errorf("%s: %q not shown:\n%s", c.port, s, out)
			}
		}
		if c.mode != "trunk" && strings.Contains(out, "Mode         trunk") {
			t.Errorf("%s shown as a trunk:\n%s", c.port, out)
		}
	}
	if bad := config(); len(bad) > 0 {
		t.Fatalf("read-only operations sent: %v", bad)
	}
}

// A dynamic port that isn't trunking gets exactly the same commands as a
// static access port, plus a warning; its mode is never changed.
func TestVlanChangeDynamicPort(t *testing.T) {
	d, c, config := driverFor(t)
	for _, port := range []string{"1/0/41", "1/0/38"} {
		sent := len(config())
		plan, err := d.PlanVlanChange(port, 110)
		if err != nil {
			t.Fatalf("%s: %v", port, err)
		}
		name := "GigabitEthernet" + port
		want := []string{"configure terminal", "interface " + name, "switchport access vlan 110", "end"}
		if got := plan.Change.Display(); strings.Join(got, "|") != strings.Join(want, "|") {
			t.Fatalf("%s: commands %v", port, got)
		}
		if len(plan.Warnings) != 1 || !strings.Contains(plan.Warnings[0], "dynamic auto mode") || !strings.Contains(plan.PortMode, "dynamic auto") {
			t.Fatalf("%s: plan %+v", port, plan)
		}
		if len(config()) != sent {
			t.Fatal("planning must not change anything")
		}
		plan.Change.Confirm(plan.Change.Display())
		if _, err := d.ApplyVlanChange(c, plan); err != nil {
			t.Fatalf("%s: %v", port, err)
		}
		after, err := d.Interface(port)
		if err != nil || after.AccessVlan != 110 || after.Mode != "dynamic auto" {
			t.Fatalf("%s after: %+v %v", port, after, err)
		}
	}
	sent := strings.Join(config(), "\n")
	if strings.Contains(sent, "switchport mode") || strings.Contains(sent, "write memory") {
		t.Fatalf("only the access VLAN may change:\n%s", sent)
	}

	// Operating as a trunk (negotiated with DTP): refused, nothing sent.
	before := len(config())
	if _, err := d.PlanVlanChange("1/0/39", 110); !errors.Is(err, ErrTrunkPort) || !strings.Contains(err.Error(), "dynamic desirable, operational: trunk") {
		t.Fatalf("negotiated trunk: %v", err)
	}
	if len(config()) != before {
		t.Fatal("a refused plan sent commands")
	}
}
