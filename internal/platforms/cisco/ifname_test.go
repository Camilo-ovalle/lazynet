// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package cisco

import "testing"

func TestNormalizeInterface(t *testing.T) {
	ok := []struct {
		in   string
		bare string
		want string
	}{
		{"1/0/1", "GigabitEthernet", "GigabitEthernet1/0/1"},
		{"gi1/0/25", "GigabitEthernet", "GigabitEthernet1/0/25"},
		{"Gi 1/0/25", "GigabitEthernet", "GigabitEthernet1/0/25"},
		{"GigabitEthernet1/0/1", "GigabitEthernet", "GigabitEthernet1/0/1"},
		{"g0/1", "GigabitEthernet", "GigabitEthernet0/1"},
		{"fa0/24", "GigabitEthernet", "FastEthernet0/24"},
		{"te1/1/1", "GigabitEthernet", "TenGigabitEthernet1/1/1"},
		{"te1/1/4", "GigabitEthernet", "TenGigabitEthernet1/1/4"}, // C9200L-48P-4X uplinks
		{"1/0/48", "GigabitEthernet", "GigabitEthernet1/0/48"},
		{"tw1/0/3", "GigabitEthernet", "TwoGigabitEthernet1/0/3"},
		{"1/1", "Ethernet", "Ethernet1/1"},
		{"eth1/49", "Ethernet", "Ethernet1/49"},
		{"po10", "Ethernet", "Port-channel10"},
		{"mgmt0", "Ethernet", "mgmt0"},
		{"gigabit1/0/2", "GigabitEthernet", "GigabitEthernet1/0/2"},
		{"gi0/0.100", "GigabitEthernet", "GigabitEthernet0/0.100"},
		{"vlan110", "GigabitEthernet", "Vlan110"},
	}
	for _, c := range ok {
		got, err := NormalizeInterface(c.in, c.bare)
		if err != nil || got != c.want {
			t.Errorf("NormalizeInterface(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	bad := []string{"", "gi", "gi1/0/1\nreload", "gi1/0/1 ; reload", "xx1/0/1", "gi1/0/1 description x", "1/0/1/1/1/1", "gi-1"}
	for _, in := range bad {
		if got, err := NormalizeInterface(in, "GigabitEthernet"); err == nil {
			t.Errorf("NormalizeInterface(%q) = %q; want error", in, got)
		}
	}
}
