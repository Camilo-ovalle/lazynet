// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package all

import (
	"slices"
	"testing"

	"github.com/Camilo-ovalle/lazynet/internal/detect"
	"github.com/Camilo-ovalle/lazynet/internal/mockssh"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/platform/platformtest"
	"github.com/Camilo-ovalle/lazynet/internal/transport"
)

// reviewedSessionCommands are the only commands, besides "show …", that
// lazynet may send outside a confirmed change. Each one only turns paging
// off (or widens the terminal) for the current session.
//
// SAFETY REVIEW: add a line here only after checking, on the vendor's
// documentation, that the command changes nothing on the device beyond the
// current CLI session. Never add anything that enters configuration mode,
// writes, commits, copies, clears or reloads.
var reviewedSessionCommands = []string{
	"set cli pager off", // PAN-OS
	"terminal length 0", // IOS, IOS-XE, NX-OS
	"terminal pager 0",  // ASA
	"terminal width 511",
}

func TestSessionCommandsAreReviewed(t *testing.T) {
	got := platform.Dialect().SessionCommands
	want := slices.Clone(reviewedSessionCommands)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("session commands changed:\n got  %q\n want %q\nsee the SAFETY REVIEW note above", got, want)
	}
}

func TestProbesAreReadOnly(t *testing.T) {
	for _, def := range platform.All() {
		for _, p := range def.Probes {
			if err := transport.CheckReadOnly(p.Command); err != nil {
				t.Errorf("%s: probe %q: %v", def.ID, p.Command, err)
			}
		}
	}
}

// Every platform can be tried without hardware: an emulator, fixtures and a
// demo device.
func TestEveryPlatformIsEmulated(t *testing.T) {
	if len(platform.All()) < 4 {
		t.Fatalf("only %d platforms registered", len(platform.All()))
	}
	for _, def := range platform.All() {
		if !mockssh.Emulates(string(def.ID)) {
			t.Errorf("%s: no mockssh emulator registered under this ID", def.ID)
		}
		if len(def.Demo) == 0 {
			t.Errorf("%s: no demo device", def.ID)
		}
	}
}

// Each emulated device is detected as its own platform, with every
// platform's probes competing, and only read-only commands reach it.
func TestDetectEveryPlatform(t *testing.T) {
	for _, def := range platform.All() {
		t.Run(string(def.ID), func(t *testing.T) {
			dev, c := platformtest.Dial(t, string(def.ID))
			got, err := detect.Identify(c, "")
			if err != nil || got.ID != def.ID {
				t.Fatalf("detected %q, %v", got.ID, err)
			}
			if err := platform.Setup(got, c); err != nil {
				t.Fatal(err)
			}
			drv := got.New(c)
			if inf, err := drv.Info("10.0.0.1"); err != nil || inf.Platform != def.ID || inf.Version == "" || inf.Model == "" {
				t.Fatalf("info = %+v, %v", inf, err)
			}
			if r, ok := drv.(platform.ConfigReader); ok {
				if cfg, err := r.RunningConfig(); err != nil || cfg == "" {
					t.Fatalf("running config: %v", err)
				}
			}
			if bad := platformtest.ConfigCommands(dev); len(bad) > 0 {
				t.Fatalf("read-only operations sent: %v", bad)
			}
		})
	}
}

func TestPinnedPlatformSkipsProbes(t *testing.T) {
	s := platformtest.Shower{Dev: mockssh.NewDevice("catalyst", "sw1"), PromptText: "sw1#"}
	def, err := detect.Identify(s, "Catalyst")
	if err != nil || def.ID != "catalyst" {
		t.Fatalf("got %q, %v", def.ID, err)
	}
	if n := len(s.Dev.Commands()); n != 0 {
		t.Fatalf("a pinned platform must not send probes, sent %d", n)
	}
	if _, err := detect.Identify(s, "junos"); err == nil {
		t.Fatal("an unknown pinned platform must be an error")
	}
}

func TestParseAliases(t *testing.T) {
	for in, want := range map[string]platform.ID{"paloalto": "paloalto", "panos": "paloalto", "PAN-OS": "paloalto", " Nexus ": "nexus", "": platform.Unknown} {
		if got, err := platform.Parse(in); err != nil || got != want {
			t.Errorf("Parse(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := platform.Parse("junos"); err == nil {
		t.Error("junos is not supported")
	}
}

func TestCapabilities(t *testing.T) {
	want := map[platform.ID][]platform.Capability{
		"catalyst": {platform.CapInterfaces, platform.CapLogs, platform.CapBackup, platform.CapVlanChange},
		"nexus":    {platform.CapInterfaces, platform.CapLogs, platform.CapBackup},
		"asa":      {platform.CapInterfaces, platform.CapLogs, platform.CapBackup},
		"paloalto": {platform.CapBackup},
	}
	for id, caps := range want {
		def, ok := platform.Lookup(id)
		if !ok {
			t.Fatalf("%s is not registered", id)
		}
		if got := def.Capabilities(); !slices.Equal(got, caps) {
			t.Errorf("%s: capabilities %v, want %v", id, got, caps)
		}
	}
	if !platform.Supports(platform.Unknown, platform.CapVlanChange) {
		t.Error("an undetected platform must not be ruled out before connecting")
	}
}
