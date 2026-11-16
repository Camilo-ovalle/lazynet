// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package catalyst

import (
	"errors"
	"strings"
	"testing"

	"github.com/Camilo-ovalle/lazynet/internal/detect"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/platform/platformtest"
	"github.com/Camilo-ovalle/lazynet/internal/transport"
)

// fixedOutput answers every command with the same text, at a Cisco prompt.
type fixedOutput string

func (f fixedOutput) Prompt() string              { return "acc-sw-01#" }
func (f fixedOutput) Show(string) (string, error) { return string(f), nil }

// Detection runs `show version | include Software`; this filters the C9200L
// fixture the same way the device would.
func TestDetectCatalyst9200L(t *testing.T) {
	var lines []string
	for _, l := range strings.Split(c9200l(t), "\n") {
		if strings.Contains(l, "Software") {
			lines = append(lines, l)
		}
	}
	def, err := detect.Run(fixedOutput(strings.Join(lines, "\n")), platform.All())
	if err != nil || def.ID != ID {
		t.Fatalf("got %q, %v", def.ID, err)
	}
}

func driverFor(t *testing.T) (*driver, *transport.Conn, func() []string) {
	t.Helper()
	dev, c := platformtest.Dial(t, "catalyst")
	def, _ := platform.Lookup(ID)
	if err := platform.Setup(def, c); err != nil {
		t.Fatal(err)
	}
	return New(c).(*driver), c, func() []string { return platformtest.ConfigCommands(dev) }
}

func TestInfoLogsAndConfig(t *testing.T) {
	d, _, config := driverFor(t)
	inf, err := d.Info("10.0.0.1")
	if err != nil || inf.Platform != ID || inf.OS != "IOS-XE" || inf.Serial == "" || inf.MAC == "" || inf.Uptime == "" || inf.Clock.IsZero() {
		t.Fatalf("info = %+v %v", inf, err)
	}
	if lr, err := d.Logs(); err != nil || len(lr.Entries) == 0 {
		t.Fatalf("logs: %v", err)
	}
	if cfg, err := d.RunningConfig(); err != nil || !strings.Contains(cfg, "hostname sw-test") {
		t.Fatalf("running-config: %v", err)
	}
	if bad := config(); len(bad) > 0 {
		t.Fatalf("read-only operations sent: %v", bad)
	}
}

func TestInterface(t *testing.T) {
	d, _, _ := driverFor(t)
	i, err := d.Interface("1/0/25")
	if err != nil || i.Name != "GigabitEthernet1/0/25" || i.Mode != "access" || i.AccessVlan != 10 || i.Description != "Camera-Parking" {
		t.Fatalf("got %+v %v", i, err)
	}
	if _, err := d.Interface("gi9/9/9"); err == nil {
		t.Fatal("unknown interface must error")
	}
}

func TestVlanChangeFlow(t *testing.T) {
	d, c, config := driverFor(t)
	dev := func() int { return len(config()) }
	plan, err := d.PlanVlanChange("1/0/1", 110)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CurrentVlan != 10 || plan.TargetVlanName != "PRINTERS" || plan.NoOp {
		t.Fatalf("plan: %+v", plan)
	}
	want := []string{"configure terminal", "interface GigabitEthernet1/0/1", "switchport access vlan 110", "end"}
	if got := plan.Change.Display(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("display: %v", got)
	}
	if dev() != 0 {
		t.Fatal("planning must not change anything")
	}
	// User declines: nothing is sent.
	if _, err := d.ApplyVlanChange(c, plan); !errors.Is(err, transport.ErrNotConfirmed) {
		t.Fatalf("unconfirmed apply: %v", err)
	}
	if dev() != 0 {
		t.Fatal("declined change reached the device")
	}
	plan.Change.Confirm(plan.Change.Display())
	if _, err := d.ApplyVlanChange(c, plan); err != nil {
		t.Fatal(err)
	}
	if after, err := d.Interface("1/0/1"); err != nil || after.AccessVlan != 110 {
		t.Fatalf("vlan not changed: %+v %v", after, err)
	}
	for _, cmd := range config() {
		if cmd == "write memory" {
			t.Fatal("must not save without asking")
		}
	}
	if got := plan.Save.Display(); len(got) != 1 || got[0] != "write memory" {
		t.Fatalf("save display: %v", got)
	}
	plan.Save.Confirm(plan.Save.Display())
	if _, err := c.Apply(plan.Save); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Same VLAN again → no-op.
	again, err := d.PlanVlanChange("Gi1/0/1", 110)
	if err != nil || !again.NoOp || again.Change != nil {
		t.Fatalf("noop: %+v %v", again, err)
	}
}

func TestVlanChangeRejections(t *testing.T) {
	d, _, config := driverFor(t)
	cases := []struct {
		port string
		vlan int
		err  error
	}{
		{"te1/1/1", 110, ErrTrunkPort},
		{"1/0/1", 300, ErrVlanNotFound},
		{"1/0/1", 1003, ErrVlanRange},
		{"1/0/1", 0, ErrVlanRange},
	}
	for _, tc := range cases {
		if _, err := d.PlanVlanChange(tc.port, tc.vlan); !errors.Is(err, tc.err) {
			t.Errorf("%s vlan %d: got %v want %v", tc.port, tc.vlan, err, tc.err)
		}
	}
	if _, err := d.PlanVlanChange("1/0/1; reload", 10); err == nil {
		t.Error("injection must be rejected")
	}
	if bad := config(); len(bad) > 0 {
		t.Fatalf("rejected plans must never send config commands: %v", bad)
	}
}
