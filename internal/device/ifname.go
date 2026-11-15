// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"fmt"
	"regexp"
	"strings"
)

var ifAliases = map[string]string{
	"g": "GigabitEthernet", "gi": "GigabitEthernet", "gig": "GigabitEthernet", "gigabitethernet": "GigabitEthernet",
	"f": "FastEthernet", "fa": "FastEthernet", "fastethernet": "FastEthernet",
	"te": "TenGigabitEthernet", "ten": "TenGigabitEthernet", "tengig": "TenGigabitEthernet", "tengigabitethernet": "TenGigabitEthernet", "tengige": "TenGigabitEthernet",
	"tw": "TwoGigabitEthernet", "twogigabitethernet": "TwoGigabitEthernet",
	"twe": "TwentyFiveGigE", "twentyfivegige": "TwentyFiveGigE",
	"fi": "FiveGigabitEthernet", "fivegigabitethernet": "FiveGigabitEthernet",
	"fo": "FortyGigabitEthernet", "fortygigabitethernet": "FortyGigabitEthernet",
	"hu": "HundredGigE", "hundredgige": "HundredGigE",
	"e": "Ethernet", "et": "Ethernet", "eth": "Ethernet", "ethernet": "Ethernet",
	"po": "Port-channel", "port-channel": "Port-channel", "portchannel": "Port-channel",
	"vl": "Vlan", "vlan": "Vlan",
	"lo": "Loopback", "loopback": "Loopback",
	"tu": "Tunnel", "tunnel": "Tunnel",
	"mgmt": "mgmt",
	"ma":   "Management", "management": "Management",
	"ap": "AppGigabitEthernet", "appgigabitethernet": "AppGigabitEthernet",
}

var (
	ifSplit  = regexp.MustCompile(`^([A-Za-z-]*)\s*(\d.*)$`)
	ifNumber = regexp.MustCompile(`^\d{1,4}(/\d{1,4}){0,3}(\.\d{1,5})?$`)
)

// NormalizeInterface expands user shorthand ("1/0/1", "gi1/0/1",
// "Gi 1/0/1") into the canonical IOS name ("GigabitEthernet1/0/1").
// Bare numbers default to GigabitEthernet (Ethernet on Nexus).
//
// The result is strictly validated because it is interpolated into device
// commands: anything other than a known prefix plus digits and slashes is
// rejected, so input like "Gi1/0/1\nreload" can never reach a device.
func NormalizeInterface(in string, p Platform) (string, error) {
	s := strings.TrimSpace(in)
	m := ifSplit.FindStringSubmatch(s)
	if m == nil {
		return "", fmt.Errorf("invalid interface %q", in)
	}
	prefix, num := strings.ToLower(m[1]), m[2]
	if !ifNumber.MatchString(num) {
		return "", fmt.Errorf("invalid interface number %q", num)
	}
	if prefix == "" {
		if p == Nexus {
			return "Ethernet" + num, nil
		}
		return "GigabitEthernet" + num, nil
	}
	if canon, ok := ifAliases[prefix]; ok {
		return canon + num, nil
	}
	// Unique-prefix fallback, e.g. "gigabit" or "fastether".
	var match string
	for alias, canon := range ifAliases {
		if len(prefix) >= 3 && strings.HasPrefix(alias, prefix) && len(alias) > 4 {
			if match != "" && match != canon {
				return "", fmt.Errorf("ambiguous interface prefix %q", m[1])
			}
			match = canon
		}
	}
	if match == "" {
		return "", fmt.Errorf("unknown interface type %q", m[1])
	}
	return match + num, nil
}
