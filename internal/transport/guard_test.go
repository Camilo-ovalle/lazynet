// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"errors"
	"testing"
)

func TestCheckReadOnly(t *testing.T) {
	allowed := []string{
		"show version",
		"show  running-config",
		"show interfaces GigabitEthernet1/0/1 switchport",
		"show version | include Software|NX-OS",
		"show logging | begin Log Buffer",
		"terminal length 0",
		"terminal pager 0",
		"terminal width 511",
		"SHOW VERSION",
		"set cli pager off",                   // PAN-OS session paging
		"set  cli  pager  off",                // whitespace is normalised
		"show system info | match sw-version", // PAN-OS filter
		"show config running",                 // PAN-OS backup
	}
	for _, c := range allowed {
		if err := CheckReadOnly(c, testDialect.SessionCommands...); err != nil {
			t.Errorf("CheckReadOnly(%q) = %v; want allowed", c, err)
		}
	}
	denied := []string{
		"configure terminal",
		"reload",
		"write memory",
		"copy running-config startup-config",
		"show version\nreload",
		"show version\rreload",
		"show run | redirect flash:x.cfg",
		"show run | tee bootflash:x",
		"show run | append flash:x",
		"show run > bootflash:x",
		"terminal monitor",
		"terminal width 80", // only the exact declared settings
		"terminal length 24",
		"clear counters",
		"debug ip packet",
		"sh run",
		"",
		// PAN-OS: only the paging setting is allowed, never configuration.
		"set cli pager on",
		"set cli config-output-format set",
		"set deviceconfig system hostname x",
		"configure",
		"commit",
		"request restart system",
		"set cli pager off; commit",
	}
	for _, c := range denied {
		if err := CheckReadOnly(c, testDialect.SessionCommands...); !errors.Is(err, ErrNotReadOnly) {
			t.Errorf("CheckReadOnly(%q) = %v; want ErrNotReadOnly", c, err)
		}
	}
	// Session settings are only allowed when a platform declares them.
	for _, c := range testDialect.SessionCommands {
		if err := CheckReadOnly(c); !errors.Is(err, ErrNotReadOnly) {
			t.Errorf("CheckReadOnly(%q) without session commands = %v; want ErrNotReadOnly", c, err)
		}
	}
}

func TestChangeRequestConfirm(t *testing.T) {
	cr := NewConfigChange("interface GigabitEthernet1/0/1", "switchport access vlan 110")
	want := []string{"configure terminal", "interface GigabitEthernet1/0/1", "switchport access vlan 110", "end"}
	if got := cr.Display(); len(got) != 4 || got[0] != want[0] || got[3] != want[3] {
		t.Fatalf("Display() = %v", got)
	}
	if cr.Confirm([]string{"configure terminal", "interface GigabitEthernet1/0/2", "switchport access vlan 110", "end"}) {
		t.Fatal("confirm must fail when shown commands differ")
	}
	if cr.Confirmed() {
		t.Fatal("must stay unconfirmed")
	}
	if !cr.Confirm(want) || !cr.Confirmed() {
		t.Fatal("confirm with exact display should succeed")
	}
	wr := NewExecChange("write memory")
	if d := wr.Display(); len(d) != 1 || d[0] != "write memory" {
		t.Fatalf("exec display = %v", d)
	}
}
