// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"errors"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/mockssh"
	"github.com/Camilo-ovalle/lazynet/internal/transport"
)

func connect(t *testing.T, platform string) (*mockssh.Device, *transport.Conn) {
	t.Helper()
	dev := mockssh.NewDevice(platform, "sw-test")
	srv, err := mockssh.Start("127.0.0.1:0", dev, "admin", "pw")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	host, port, _ := net.SplitHostPort(srv.Addr())
	p, _ := strconv.Atoi(port)
	o := transport.Options{Host: host, Port: p, Username: "admin", Password: "pw",
		KnownHostsFile: filepath.Join(t.TempDir(), "kh"), DialTimeout: 5 * time.Second, CommandTimeout: 10 * time.Second}
	_, err = transport.Dial(o)
	var uk *transport.UnknownHostKeyError
	if !errors.As(err, &uk) {
		t.Fatalf("expected unknown key, got %v", err)
	}
	if err := transport.TrustHostKey(o.KnownHostsFile, uk); err != nil {
		t.Fatal(err)
	}
	c, err := transport.Dial(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return dev, c
}

func countConfig(dev *mockssh.Device) int {
	n := 0
	for _, c := range dev.Commands() {
		if strings.HasPrefix(c, "configure") || strings.HasPrefix(c, "switchport") || strings.HasPrefix(c, "interface") {
			n++
		}
	}
	return n
}

func TestDetectAndInfoAllPlatforms(t *testing.T) {
	for _, tc := range []struct {
		platform string
		want     Platform
		os       string
	}{{"catalyst", Catalyst, "IOS-XE"}, {"nexus", Nexus, "NX-OS"}, {"asa", ASA, "ASA"}} {
		dev, c := connect(t, tc.platform)
		p, err := Detect(c)
		if err != nil || p != tc.want {
			t.Fatalf("%s: detect = %v %v", tc.platform, p, err)
		}
		if err := Setup(p, c); err != nil {
			t.Fatalf("%s: setup: %v", tc.platform, err)
		}
		d, _ := New(p, c)
		inf, err := d.Info("10.0.0.1")
		if err != nil || inf.OS != tc.os || inf.Serial == "" || inf.MAC == "" || inf.Uptime == "" || inf.Clock.IsZero() {
			t.Fatalf("%s: info = %+v %v", tc.platform, inf, err)
		}
		lr, err := d.Logs()
		if err != nil || len(lr.Entries) == 0 {
			t.Fatalf("%s: logs: %v", tc.platform, err)
		}
		cfg, err := d.RunningConfig()
		if err != nil || !strings.Contains(cfg, "hostname sw-test") {
			t.Fatalf("%s: running-config: %v", tc.platform, err)
		}
		if countConfig(dev) != 0 {
			t.Fatalf("%s: read-only operations sent config commands: %v", tc.platform, dev.Commands())
		}
	}
}

func TestInterfaceCatalyst(t *testing.T) {
	_, c := connect(t, "catalyst")
	d, _ := New(Catalyst, c)
	i, err := d.Interface("1/0/25")
	if err != nil || i.Name != "GigabitEthernet1/0/25" || i.Mode != "access" || i.AccessVlan != 10 || i.Description != "Camera-Parking" {
		t.Fatalf("got %+v %v", i, err)
	}
	if _, err := d.Interface("gi9/9/9"); err == nil {
		t.Fatal("unknown interface must error")
	}
}

func TestVlanChangeFlow(t *testing.T) {
	dev, c := connect(t, "catalyst")
	plan, err := PlanVlanChange(c, Catalyst, "1/0/1", 110)
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
	if countConfig(dev) != 0 {
		t.Fatal("planning must not change anything")
	}
	// User declines: nothing is sent.
	if _, err := ApplyVlanChange(c, plan); !errors.Is(err, transport.ErrNotConfirmed) {
		t.Fatalf("unconfirmed apply: %v", err)
	}
	if dev.PortVlan("GigabitEthernet1/0/1") != 10 || countConfig(dev) != 0 {
		t.Fatal("declined change reached the device")
	}
	plan.Change.Confirm(plan.Change.Display())
	if _, err := ApplyVlanChange(c, plan); err != nil {
		t.Fatal(err)
	}
	if dev.PortVlan("GigabitEthernet1/0/1") != 110 {
		t.Fatal("vlan not changed")
	}
	if dev.Saved() {
		t.Fatal("must not save without asking")
	}
	save := SaveRequest(Catalyst)
	save.Confirm(save.Display())
	if _, err := c.Apply(save); err != nil || !dev.Saved() {
		t.Fatalf("save: %v", err)
	}
	// Same VLAN again → no-op.
	again, err := PlanVlanChange(c, Catalyst, "Gi1/0/1", 110)
	if err != nil || !again.NoOp || again.Change != nil {
		t.Fatalf("noop: %+v %v", again, err)
	}
}

func TestVlanChangeRejections(t *testing.T) {
	dev, c := connect(t, "catalyst")
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
		if _, err := PlanVlanChange(c, Catalyst, tc.port, tc.vlan); !errors.Is(err, tc.err) {
			t.Errorf("%s vlan %d: got %v want %v", tc.port, tc.vlan, err, tc.err)
		}
	}
	if _, err := PlanVlanChange(c, Catalyst, "1/0/1; reload", 10); err == nil {
		t.Error("injection must be rejected")
	}
	if _, err := PlanVlanChange(c, Nexus, "1/1", 10); !errors.Is(err, ErrUnsupported) {
		t.Errorf("nexus: %v", err)
	}
	if countConfig(dev) != 0 {
		t.Fatal("rejected plans must never send config commands")
	}
}
