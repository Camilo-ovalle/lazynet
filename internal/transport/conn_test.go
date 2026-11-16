// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"errors"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/mockssh"
)

func startMock(t *testing.T, platform string, unpriv bool) (*mockssh.Server, Options) {
	t.Helper()
	dev := mockssh.NewDevice(platform, "lab-"+platform)
	dev.Unprivileged = unpriv
	srv, err := mockssh.Start("127.0.0.1:0", dev, "admin", "secret")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	host, port, _ := net.SplitHostPort(srv.Addr())
	p, _ := strconv.Atoi(port)
	return srv, Options{
		Host: host, Port: p, Username: "admin", Password: "secret",
		KnownHostsFile: filepath.Join(t.TempDir(), "known_hosts"),
		DialTimeout:    5 * time.Second, CommandTimeout: 10 * time.Second, LegacySSH: true,
	}
}

// dialTrusted performs trust-on-first-use the way the UI does.
func dialTrusted(t *testing.T, o Options) *Conn {
	t.Helper()
	_, err := Dial(o)
	var uk *UnknownHostKeyError
	if !errors.As(err, &uk) {
		t.Fatalf("first dial should report unknown host key, got %v", err)
	}
	if err := TrustHostKey(o.KnownHostsFile, uk); err != nil {
		t.Fatal(err)
	}
	c, err := Dial(o)
	if err != nil {
		t.Fatalf("dial after trust: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestDialShowAndGuard(t *testing.T) {
	srv, o := startMock(t, "catalyst", false)
	c := dialTrusted(t, o)
	if c.Hostname() != "lab-catalyst" {
		t.Fatalf("hostname = %q", c.Hostname())
	}
	out, err := c.Show("show version | include Software")
	if err != nil || !strings.Contains(out, "Cisco IOS XE Software") {
		t.Fatalf("show version: %q %v", out, err)
	}
	if _, err := c.Show("configure terminal"); !errors.Is(err, ErrNotReadOnly) {
		t.Fatalf("guard: %v", err)
	}
	for _, cmd := range srv.Device.Commands() {
		if strings.HasPrefix(cmd, "conf") {
			t.Fatal("guarded command reached the device")
		}
	}
}

func TestApplyRequiresConfirmation(t *testing.T) {
	srv, o := startMock(t, "catalyst", false)
	c := dialTrusted(t, o)
	cr := NewConfigChange("interface GigabitEthernet1/0/1", "switchport access vlan 110")
	if _, err := c.Apply(cr); !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("unconfirmed apply: %v", err)
	}
	if srv.Device.PortVlan("GigabitEthernet1/0/1") != 10 {
		t.Fatal("unconfirmed change modified the device")
	}
	cr.Confirm(cr.Display())
	if _, err := c.Apply(cr); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if v := srv.Device.PortVlan("GigabitEthernet1/0/1"); v != 110 {
		t.Fatalf("vlan after apply = %d", v)
	}
	// Session must be back at privileged exec, able to run show commands.
	if _, err := c.Show("show clock"); err != nil {
		t.Fatalf("show after config: %v", err)
	}
}

func TestDialErrors(t *testing.T) {
	_, o := startMock(t, "catalyst", true)
	dialTrusted := func(o Options) error {
		_, err := Dial(o)
		var uk *UnknownHostKeyError
		if errors.As(err, &uk) {
			_ = TrustHostKey(o.KnownHostsFile, uk)
			_, err = Dial(o)
		}
		return err
	}
	if err := dialTrusted(o); !errors.Is(err, ErrNotPrivileged) {
		t.Fatalf("unprivileged login: %v", err)
	}
	bad := o
	bad.Password = "wrong"
	if _, err := Dial(bad); !errors.Is(err, ErrAuth) {
		t.Fatalf("bad password: %v", err)
	}
}

func TestHostKeyChanged(t *testing.T) {
	srv1, o := startMock(t, "catalyst", false)
	_ = dialTrusted(t, o)
	addr := srv1.Addr()
	_ = srv1.Close()
	dev := mockssh.NewDevice("catalyst", "impostor")
	srv2, err := mockssh.Start(addr, dev, "admin", "secret")
	if err != nil {
		t.Skipf("could not rebind %s: %v", addr, err)
	}
	defer srv2.Close()
	if _, err := Dial(o); !errors.Is(err, ErrHostKeyChanged) {
		t.Fatalf("expected host key changed, got %v", err)
	}
}

func TestCheckHostKeyDoesNotAuthenticate(t *testing.T) {
	srv, o := startMock(t, "catalyst", false)
	o.Password = ""
	err := CheckHostKey(o)
	var uk *UnknownHostKeyError
	if !errors.As(err, &uk) {
		t.Fatalf("want unknown key, got %v", err)
	}
	if IsKnown(o.KnownHostsFile, uk) {
		t.Fatal("not trusted yet")
	}
	if err := TrustHostKey(o.KnownHostsFile, uk); err != nil {
		t.Fatal(err)
	}
	if !IsKnown(o.KnownHostsFile, uk) {
		t.Fatal("should be known after trust")
	}
	if err := CheckHostKey(o); err != nil {
		t.Fatalf("known host: %v", err)
	}
	if len(srv.Device.Commands()) != 0 {
		t.Fatal("probe must not open a session")
	}
	o.Port = 1 // nothing listens there
	if err := CheckHostKey(o); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("unreachable: %v", err)
	}
}

func TestPANOSPrompt(t *testing.T) {
	for p, want := range map[string]bool{
		"admin@pa-fw1>":         true,
		"admin@pa-fw1(active)>": true,
		"ops.user@PA-440-01>":   true,
		"admin@pa-fw1>  ":       true,
		"admin@pa-fw1#":         false, // PAN-OS configure mode: never used
		"core-sw1>":             false, // Cisco user EXEC must stay refused
		"core-sw1#":             false,
		"fw/admin>":             false,
		"admin@pa fw1>":         false,
	} {
		if got := IsPANOSPrompt(p); got != want {
			t.Errorf("IsPANOSPrompt(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestDialPANOS(t *testing.T) {
	srv, o := startMock(t, "paloalto", false)
	c := dialTrusted(t, o)
	if c.Prompt() != "admin@lab-paloalto>" || c.Hostname() != "lab-paloalto" {
		t.Fatalf("prompt %q hostname %q", c.Prompt(), c.Hostname())
	}
	if _, err := c.Show("set cli pager off"); err != nil {
		t.Fatal(err)
	}
	out, err := c.Show("show system info")
	if err != nil || !strings.Contains(out, "sw-version: 11.1.4-h7") {
		t.Fatalf("show system info: %v\n%s", err, out)
	}
	if _, err := c.Show("show nonsense"); err == nil {
		t.Fatal("PAN-OS 'Invalid syntax.' must be reported as a failure")
	}
	if _, err := c.Show("configure"); !errors.Is(err, ErrNotReadOnly) {
		t.Fatalf("configure must be blocked by the guard, got %v", err)
	}
	// Defense in depth: even a confirmed change is refused on PAN-OS.
	cr := NewConfigChange("set deviceconfig system hostname x")
	cr.Confirm(cr.Display())
	if _, err := c.Apply(cr); !errors.Is(err, ErrChangesUnsupported) {
		t.Fatalf("Apply on PAN-OS = %v, want ErrChangesUnsupported", err)
	}
	for _, cmd := range srv.Device.Commands() {
		if strings.HasPrefix(cmd, "configure") || strings.HasPrefix(cmd, "set deviceconfig") || cmd == "enable" {
			t.Fatalf("unexpected command reached the firewall: %q", cmd)
		}
	}
}
