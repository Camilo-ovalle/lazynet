// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package device

import "testing"

func TestNormalizeInterface(t *testing.T) {
	ok := []struct {
		in   string
		p    Platform
		want string
	}{
		{"1/0/1", Catalyst, "GigabitEthernet1/0/1"},
		{"gi1/0/25", Catalyst, "GigabitEthernet1/0/25"},
		{"Gi 1/0/25", Catalyst, "GigabitEthernet1/0/25"},
		{"GigabitEthernet1/0/1", Catalyst, "GigabitEthernet1/0/1"},
		{"g0/1", Catalyst, "GigabitEthernet0/1"},
		{"fa0/24", Catalyst, "FastEthernet0/24"},
		{"te1/1/1", Catalyst, "TenGigabitEthernet1/1/1"},
		{"tw1/0/3", Catalyst, "TwoGigabitEthernet1/0/3"},
		{"1/1", Nexus, "Ethernet1/1"},
		{"eth1/49", Nexus, "Ethernet1/49"},
		{"po10", Nexus, "Port-channel10"},
		{"mgmt0", Nexus, "mgmt0"},
		{"gigabit1/0/2", Catalyst, "GigabitEthernet1/0/2"},
		{"gi0/0.100", ASA, "GigabitEthernet0/0.100"},
		{"vlan110", Catalyst, "Vlan110"},
	}
	for _, c := range ok {
		got, err := NormalizeInterface(c.in, c.p)
		if err != nil || got != c.want {
			t.Errorf("NormalizeInterface(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	bad := []string{"", "gi", "gi1/0/1\nreload", "gi1/0/1 ; reload", "xx1/0/1", "gi1/0/1 description x", "1/0/1/1/1/1", "gi-1"}
	for _, in := range bad {
		if got, err := NormalizeInterface(in, Catalyst); err == nil {
			t.Errorf("NormalizeInterface(%q) = %q; want error", in, got)
		}
	}
}
