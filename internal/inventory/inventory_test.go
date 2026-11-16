// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package inventory

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadResolveGroups(t *testing.T) {
	p := filepath.Join(t.TempDir(), "inv.yml")
	if err := os.WriteFile(p, []byte(Example), 0o600); err != nil {
		t.Fatal(err)
	}
	inv, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if errs := inv.Validate(nil); len(errs) > 0 {
		t.Fatalf("unexpected validation errors: %v", errs)
	}
	d := inv.Resolve("CORE-SW1")
	if d.Host != "192.168.100.100" || d.Username != "admin" {
		t.Fatalf("resolve by name: %+v", d)
	}
	if fw := inv.Resolve("192.168.100.1"); fw.Username != "fwadmin" {
		t.Fatalf("per-device username should win: %+v", fw)
	}
	if d := inv.Resolve("192.168.100.1:22"); d.Name != "fw-edge" {
		t.Fatalf("resolve host:port: %+v", d)
	}
	adhoc := inv.Resolve("10.9.9.9:2222")
	if adhoc.Host != "10.9.9.9" || adhoc.Port != 2222 || adhoc.Username != "admin" {
		t.Fatalf("ad-hoc: %+v", adhoc)
	}
	if got := len(inv.Group("core")); got != 1 {
		t.Fatalf("group core: %d", got)
	}
	if g := inv.Groups(); strings.Join(g, ",") != "access,core,datacenter,firewalls" {
		t.Fatalf("groups: %v", g)
	}
}

func TestAddPreservesComments(t *testing.T) {
	p := filepath.Join(t.TempDir(), "inv.yml")
	if err := os.WriteFile(p, []byte(Example), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Add(p, Device{Name: "new-sw", Host: "10.1.1.1", Group: "lab"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "# lazynet inventory") {
		t.Fatal("comments were lost")
	}
	inv, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Resolve("new-sw").Host != "10.1.1.1" {
		t.Fatal("device not added")
	}
}

func TestAddToMissingFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "inv.yml")
	if err := Add(p, Device{Host: "10.1.1.1"}); err != nil {
		t.Fatal(err)
	}
	inv, err := Load(p)
	if err != nil || len(inv.Devices) != 1 {
		t.Fatalf("got %v %v", inv, err)
	}
}

func TestValidate(t *testing.T) {
	inv := &Inventory{Devices: []Device{{Name: "a", Host: "1"}, {Name: "A", Host: "2"}, {Name: "b"}, {Name: "c", Host: "3", Platform: "junos"}}}
	known := func(p string) error {
		if p != "catalyst" {
			return errors.New("unknown platform")
		}
		return nil
	}
	if n := len(inv.Validate(known)); n != 3 {
		t.Fatalf("want 3 errors, got %d: %v", n, inv.Validate(known))
	}
}
