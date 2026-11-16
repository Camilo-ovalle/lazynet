// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package cisco holds what the Cisco platforms (catalyst, nexus, asa) share:
// failure markers, the detection probe command, interface-name
// normalization and the clock/log/interface readers. It registers nothing;
// each platform package does.
package cisco

import (
	"strings"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
)

// FailedWhenContains are the error markers of IOS, IOS-XE, NX-OS and ASA.
var FailedWhenContains = []string{
	"% Ambiguous command",
	"% Incomplete command",
	"% Invalid input detected",
	"% Unknown command",
	"% Invalid command",
	"ERROR: %",
	"% Authorization failed",
}

// VersionProbe is the detection command shared by the Cisco platforms. It is
// filtered on the device so it is short enough not to page before terminal
// settings are applied.
const VersionProbe = "show version | include Software"

// TerminalSetup turns paging off on IOS, IOS-XE and NX-OS.
var TerminalSetup = []string{"terminal length 0", "terminal width 511"}

// PrivilegedPrompt reports whether prompt is a privileged-EXEC prompt
// ("core-sw1#"), the only Cisco prompt lazynet works at.
func PrivilegedPrompt(prompt string) bool { return strings.HasSuffix(strings.TrimSpace(prompt), "#") }

// ContainsAny reports whether out contains any of subs (for probe matches).
func ContainsAny(out string, subs ...string) bool {
	for _, s := range subs {
		if strings.Contains(out, s) {
			return true
		}
	}
	return false
}

// Clock reads `show clock`; the zero time if it cannot be parsed.
func Clock(s platform.Shower) time.Time {
	out, err := s.Show("show clock")
	if err != nil {
		return time.Time{}
	}
	t, _ := parse.ParseClock(out)
	return t
}

// Logs reads the log buffer with cmd and parses it against the device clock.
func Logs(s platform.Shower, cmd string) (platform.LogResult, error) {
	now := Clock(s)
	out, err := s.Show(cmd)
	if err != nil {
		return platform.LogResult{}, err
	}
	return platform.LogResult{Entries: parse.ParseLogs(out, now), DeviceClock: now, Command: cmd}, nil
}

// Interface normalizes name (bare numbers get the bare prefix) and runs
// "<cmd> <name>".
func Interface(s platform.Shower, name, bare, cmd string) (platform.InterfaceDetail, error) {
	n, err := NormalizeInterface(name, bare)
	if err != nil {
		return platform.InterfaceDetail{}, err
	}
	out, err := s.Show(cmd + " " + n)
	if err != nil {
		return platform.InterfaceDetail{}, err
	}
	return platform.InterfaceDetail{Interface: parse.ParseInterface(out), Raw: out}, nil
}
