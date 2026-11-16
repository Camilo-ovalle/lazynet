// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/Camilo-ovalle/lazynet/internal/config"
	"github.com/Camilo-ovalle/lazynet/internal/creds"
	"github.com/Camilo-ovalle/lazynet/internal/device"
	"github.com/Camilo-ovalle/lazynet/internal/inventory"
	"github.com/Camilo-ovalle/lazynet/internal/mockssh"
)

type fakePrompter struct {
	passwords []string
	trust     bool
	pwCalls   int
	keyCalls  int
}

func (f *fakePrompter) Username(string) (string, error) { return "admin", nil }
func (f *fakePrompter) Password(_, _ string, _ bool) (string, error) {
	f.pwCalls++
	if len(f.passwords) == 0 {
		return "", errors.New("no more passwords")
	}
	p := f.passwords[0]
	f.passwords = f.passwords[1:]
	return p, nil
}
func (f *fakePrompter) TrustHostKey(_, _, _ string) (bool, error) { f.keyCalls++; return f.trust, nil }

func setup(t *testing.T) (*App, inventory.Device, *mockssh.Device) {
	t.Helper()
	a, err := LoadFrom(config.PathsUnder(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	a.Creds = creds.NewManager(0) // keep tests out of the real keyring
	dev := mockssh.NewDevice("catalyst", "sw1")
	srv, err := mockssh.Start("127.0.0.1:0", dev, "admin", "good")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	h, p, _ := net.SplitHostPort(srv.Addr())
	port, _ := strconv.Atoi(p)
	return a, inventory.Device{Name: "sw1", Host: h, Port: port}, dev
}

func TestConnectRetriesAndCaches(t *testing.T) {
	a, d, _ := setup(t)
	pr := &fakePrompter{passwords: []string{"bad", "good"}, trust: true}
	s, err := a.Connect(d, pr)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Platform != device.Catalyst || pr.pwCalls != 2 || pr.keyCalls != 1 {
		t.Fatalf("platform=%v pw=%d key=%d", s.Platform, pr.pwCalls, pr.keyCalls)
	}
	// Second connect: cached password, known host key → no prompts.
	s2, err := a.Connect(d, pr)
	if err != nil {
		t.Fatal(err)
	}
	s2.Close()
	if pr.pwCalls != 2 || pr.keyCalls != 1 {
		t.Fatalf("expected no new prompts, pw=%d key=%d", pr.pwCalls, pr.keyCalls)
	}
	if a.DetectedPlatforms()["sw1"] != device.Catalyst {
		t.Fatal("detection not cached")
	}
}

func TestConnectRejectKey(t *testing.T) {
	a, d, _ := setup(t)
	if _, err := a.Connect(d, &fakePrompter{passwords: []string{"good"}, trust: false}); !errors.Is(err, ErrHostKeyRejected) {
		t.Fatalf("got %v", err)
	}
}

func TestVlanAuditAndBackup(t *testing.T) {
	a, d, dev := setup(t)
	s, err := a.Connect(d, &fakePrompter{passwords: []string{"good"}, trust: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r1, err := a.Backup(s)
	if err != nil || !r1.Changed {
		t.Fatalf("backup: %+v %v", r1, err)
	}
	plan, err := a.PlanVlan(s, "1/0/2", 20)
	if err != nil {
		t.Fatal(err)
	}
	plan.Change.Confirm(plan.Change.Display())
	if err := a.ApplyVlan(s, plan); err != nil {
		t.Fatal(err)
	}
	if dev.PortVlan("Gi1/0/2") != 20 {
		t.Fatal("not applied")
	}
	r2, err := a.Backup(s)
	if err != nil || !r2.Changed {
		t.Fatalf("second backup should differ: %+v %v", r2, err)
	}
	st, _ := a.Backups()
	diff, err := st.Diff("sw1", "", "")
	if err != nil || diff == "" {
		t.Fatalf("diff: %q %v", diff, err)
	}
}

func TestDemo(t *testing.T) {
	a, stop, err := StartDemo()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	res := a.ForEach(a.Inv.Devices, func(d inventory.Device) (any, error) {
		s, err := a.Connect(d, &fakePrompter{})
		if err != nil {
			return nil, err
		}
		defer s.Close()
		return s.Driver.Info(d.Host)
	})
	for _, r := range res {
		if r.Err != nil {
			t.Fatalf("%s: %v", r.Device.Name, r.Err)
		}
	}
}

func TestDemoPaloAltoBackup(t *testing.T) {
	a, stop, err := StartDemo()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	d := a.Inv.Resolve("pa-fw1")
	s, err := a.Connect(d, &fakePrompter{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Platform != device.PaloAlto {
		t.Fatalf("platform = %q", s.Platform)
	}
	res, err := a.Backup(s)
	if err != nil || !res.Changed {
		t.Fatalf("backup: %+v %v", res, err)
	}
	store, err := a.Backups()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := store.Content(d.Label(), res.Hash)
	if err != nil || !strings.Contains(cfg, `<config version="11.1.0"`) {
		t.Fatalf("stored backup: %v\n%.200s", err, cfg)
	}
}
