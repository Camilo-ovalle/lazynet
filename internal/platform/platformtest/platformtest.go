// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package platformtest helps platform packages test their drivers against
// the mockssh emulators, over real SSH or in memory.
package platformtest

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/mockssh"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/transport"
)

// FixedNow is the clock of emulated devices built by Device.
var FixedNow = time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

// Device builds an emulated device whose clock is FixedNow.
func Device(emulator, hostname string) *mockssh.Device {
	d := mockssh.NewDevice(emulator, hostname)
	d.Now = func() time.Time { return FixedNow }
	return d
}

// Dial starts the emulator over SSH and opens a trusted session with the
// dialect of the platforms registered in the test binary, the same way the
// app does.
func Dial(t testing.TB, emulator string) (*mockssh.Device, *transport.Conn) {
	t.Helper()
	dev := mockssh.NewDevice(emulator, "sw-test")
	srv, err := mockssh.Start("127.0.0.1:0", dev, "admin", "pw")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	host, port, _ := net.SplitHostPort(srv.Addr())
	p, _ := strconv.Atoi(port)
	o := transport.Options{
		Host: host, Port: p, Username: "admin", Password: "pw",
		KnownHostsFile: filepath.Join(t.TempDir(), "known_hosts"),
		DialTimeout:    5 * time.Second, CommandTimeout: 10 * time.Second,
		Dialect: platform.Dialect(),
	}
	_, err = transport.Dial(o)
	var uk *transport.UnknownHostKeyError
	if !errors.As(err, &uk) {
		t.Fatalf("expected an unknown host key, got %v", err)
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

// Shower runs commands on an emulated device in memory and reports a login
// prompt, like a *transport.Conn. Like the transport, it refuses anything
// the read-only guard refuses, and reports outputs with a failure marker as
// transport.ErrRejected.
type Shower struct {
	Dev        *mockssh.Device
	PromptText string
}

// Prompt is the login prompt.
func (s Shower) Prompt() string { return s.PromptText }

// Show runs cmd on the emulated device.
func (s Shower) Show(cmd string) (string, error) {
	d := platform.Dialect()
	if err := transport.CheckReadOnly(cmd, d.SessionCommands...); err != nil {
		return "", err
	}
	out := s.Dev.Exec(cmd)
	for _, m := range d.FailedWhenContains {
		if strings.Contains(out, m) {
			return out, fmt.Errorf("%s: %w: %s", cmd, transport.ErrRejected, out)
		}
	}
	return out, nil
}

// ConfigCommands returns the commands the device received that are not
// "show", the platform's session setup or "exit": anything a read-only
// operation must never send.
func ConfigCommands(dev *mockssh.Device) []string {
	setup := platform.Dialect().SessionCommands
	var bad []string
	for _, c := range dev.Commands() {
		if transport.CheckReadOnly(c, setup...) != nil && c != "exit" {
			bad = append(bad, c)
		}
	}
	return bad
}
