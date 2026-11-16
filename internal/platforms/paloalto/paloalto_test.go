// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package paloalto

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/detect"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/platform/platformtest"
	"github.com/Camilo-ovalle/lazynet/internal/transport"
)

func TestIsPrompt(t *testing.T) {
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
		if got := IsPrompt(p); got != want {
			t.Errorf("IsPrompt(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestParseSystemInfo(t *testing.T) {
	si := ParseSystemInfo(platformtest.Device("paloalto", "lab1").Exec("show system info"))
	if si.Hostname != "lab1" || si.Model != "PA-440" || si.Serial != "024101001234" || si.Version != "11.1.4-h7" ||
		si.MAC != "00:50:56:a1:b2:c3" || si.IP != "10.10.0.2" || si.Uptime != "41 days, 6:12:40" {
		t.Fatalf("got %+v", si)
	}
	if !si.Clock.Equal(platformtest.FixedNow.Truncate(time.Second)) {
		t.Fatalf("clock = %v, want %v", si.Clock, platformtest.FixedNow)
	}
	// "unknown" values are treated as empty.
	if ParseSystemInfo("serial: unknown\nip-address: unknown\n").Serial != "" {
		t.Fatal("unknown serial should be empty")
	}
}

func TestDetectAndDriver(t *testing.T) {
	s := platformtest.Shower{Dev: platformtest.Device("paloalto", "pa-fw1"), PromptText: "admin@pa-fw1>"}
	def, err := detect.Run(s, platform.All())
	if err != nil || def.ID != ID {
		t.Fatalf("detect = %q, %v", def.ID, err)
	}
	if err := platform.Setup(def, s); err != nil {
		t.Fatal(err)
	}
	drv := def.New(s)
	info, err := drv.Info("192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	if info.Hostname != "pa-fw1" || info.Model != "PA-440" || info.OS != "PAN-OS" || info.Version != "11.1.4-h7" ||
		info.Serial != "024101001234" || info.IP != "10.10.0.2" || info.Clock.IsZero() {
		t.Fatalf("info = %+v", info)
	}
	cfg, err := drv.(platform.ConfigReader).RunningConfig()
	if err != nil || !strings.HasPrefix(strings.TrimSpace(cfg), "<config version=") || !strings.Contains(cfg, "<hostname>pa-fw1</hostname>") {
		t.Fatalf("running config: %v\n%s", err, cfg)
	}
	// Info and backups only, for now.
	for _, c := range []platform.Capability{platform.CapInterfaces, platform.CapLogs, platform.CapVlanChange} {
		if def.Supports(c) {
			t.Errorf("Palo Alto must not offer %s yet", c)
		}
		if err := platform.Unsupported(ID, c); !errors.Is(err, platform.ErrUnsupported) || !strings.Contains(err.Error(), "available: device info, configuration backups") {
			t.Errorf("Unsupported(%s) = %v", c, err)
		}
	}
	if bad := platformtest.ConfigCommands(s.Dev); len(bad) > 0 {
		t.Fatalf("unexpected commands sent: %v", bad)
	}
}

func TestSessionIsReadOnly(t *testing.T) {
	dev, c := platformtest.Dial(t, "paloalto")
	if !c.ReadOnly() || c.Hostname() != "sw-test" {
		t.Fatalf("read-only %v, hostname %q", c.ReadOnly(), c.Hostname())
	}
	if _, err := c.Show("show nonsense"); !errors.Is(err, transport.ErrRejected) {
		t.Fatalf("'Invalid syntax.' must be reported as a rejection, got %v", err)
	}
	cr := transport.NewConfigChange("set deviceconfig system hostname x")
	cr.Confirm(cr.Display())
	if _, err := c.Apply(cr); !errors.Is(err, transport.ErrChangesUnsupported) {
		t.Fatalf("Apply = %v, want ErrChangesUnsupported", err)
	}
	if bad := platformtest.ConfigCommands(dev); len(bad) > 0 {
		t.Fatalf("unexpected commands sent: %v", bad)
	}
}
