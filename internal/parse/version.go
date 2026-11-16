// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package parse holds the parsers and data types shared by several platforms
// (interfaces, switchports, logs, clocks, MACs). Parsers for a single
// platform live in that platform's package (internal/platforms/...).
// Parsers are lenient: they extract what they recognise and leave fields
// empty otherwise, so callers can always fall back to showing the raw output.
package parse

import (
	"regexp"
	"strings"
)

// Version is what `show version` (plus helpers) tells us about a device.
type Version struct {
	Hostname string
	OS       string // e.g. "IOS-XE", "IOS", "NX-OS", "ASA"
	Version  string
	Model    string
	Serial   string
	MAC      string
	Uptime   string
}

// Find returns the first submatch of re in s, trimmed, or "".
func Find(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

var (
	boardID      = regexp.MustCompile(`(?im)Processor board ID (\S+)`)
	invSN        = regexp.MustCompile(`SN:\s*(\S+)`)
	ifaceMAC     = regexp.MustCompile(`(?i)address(?: is|:) ([0-9a-f]{4}\.[0-9a-f]{4}\.[0-9a-f]{4})`)
	macSeparator = regexp.MustCompile(`[^0-9a-f]`)
)

// BoardID returns the "Processor board ID" of Cisco `show version` (the
// chassis serial on IOS and NX-OS).
func BoardID(out string) string { return Find(boardID, out) }

// InventorySerial returns the first serial number in `show inventory`
// (the chassis / stack on every platform).
func InventorySerial(out string) string { return Find(invSN, out) }

// InterfaceMAC extracts the burned-in address from `show interface X`.
func InterfaceMAC(out string) string { return NormalizeMAC(Find(ifaceMAC, out)) }

// NormalizeMAC renders any MAC notation in Cisco dotted form (aaaa.bbbb.cccc).
func NormalizeMAC(s string) string {
	h := macSeparator.ReplaceAllString(strings.ToLower(s), "")
	if len(h) != 12 {
		return s
	}
	return h[0:4] + "." + h[4:8] + "." + h[8:12]
}
