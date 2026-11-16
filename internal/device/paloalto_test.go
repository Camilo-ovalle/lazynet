// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"errors"
	"strings"
	"testing"

	"github.com/Camilo-ovalle/lazynet/internal/mockssh"
)

// panShower runs commands against the emulated firewall and reports a PAN-OS
// prompt, like a *transport.Conn logged in to a Palo Alto.
type panShower struct {
	dev    *mockssh.Device
	prompt string
}

func (p panShower) Prompt() string { return p.prompt }
func (p panShower) Show(cmd string) (string, error) {
	out := p.dev.Exec(cmd)
	if out == "Invalid syntax." {
		return out, errors.New("device rejected command")
	}
	return out, nil
}

func TestPaloAltoDetectAndDriver(t *testing.T) {
	s := panShower{mockssh.NewDevice("paloalto", "pa-fw1"), "admin@pa-fw1>"}
	p, err := Detect(s)
	if err != nil || p != PaloAlto {
		t.Fatalf("Detect = %q, %v", p, err)
	}
	if err := Setup(p, s); err != nil {
		t.Fatal(err)
	}
	drv, err := New(p, s)
	if err != nil {
		t.Fatal(err)
	}
	info, err := drv.Info("192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if info.Hostname != "pa-fw1" || info.Model != "PA-440" || info.OS != "PAN-OS" || info.Version != "11.1.4-h7" ||
		info.Serial != "024101001234" || info.IP != "10.10.0.2" || info.Clock.IsZero() {
		t.Fatalf("info = %+v", info)
	}
	cfg, err := drv.RunningConfig()
	if err != nil || !strings.HasPrefix(strings.TrimSpace(cfg), "<config version=") || !strings.Contains(cfg, "<hostname>pa-fw1</hostname>") {
		t.Fatalf("running config: %v\n%s", err, cfg)
	}
	if _, err := drv.Interface("ethernet1/1"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Interface = %v, want ErrUnsupported", err)
	}
	if _, err := drv.Logs(); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Logs = %v, want ErrUnsupported", err)
	}
	if _, err := PlanVlanChange(s, PaloAlto, "ethernet1/1", 20); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("PlanVlanChange = %v, want ErrUnsupported", err)
	}
	// Only read-only commands and the paging setting reached the firewall.
	for _, c := range s.dev.Commands() {
		if !strings.HasPrefix(c, "show ") && c != "set cli pager off" {
			t.Fatalf("unexpected command sent: %q", c)
		}
	}
}

// A Cisco-style prompt never takes the PAN-OS path.
func TestDetectIgnoresPANOSPathForCisco(t *testing.T) {
	s := panShower{mockssh.NewDevice("catalyst", "sw1"), "sw1#"}
	if p, err := Detect(s); err != nil || p != Catalyst {
		t.Fatalf("Detect = %q, %v", p, err)
	}
}

func TestParsePlatformAliases(t *testing.T) {
	for _, in := range []string{"paloalto", "panos", "pan-os"} {
		if p, err := ParsePlatform(in); err != nil || p != PaloAlto {
			t.Errorf("ParsePlatform(%q) = %q, %v", in, p, err)
		}
	}
}
