// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package parse

import (
	"regexp"
	"strconv"
	"strings"
)

// Interface is the useful subset of `show interface(s) X`.
type Interface struct {
	Name         string `json:"name"`
	Status       string `json:"status"` // up | down | administratively down
	Protocol     string `json:"protocol"`
	Reason       string `json:"reason"` // connected | notconnect | err-disabled | ...
	Description  string `json:"description"`
	MAC          string `json:"mac"`
	Duplex       string `json:"duplex"`
	Speed        string `json:"speed"`
	Mode         string `json:"mode"` // access | trunk | routed (when known)
	AccessVlan   int    `json:"access_vlan"`
	InputErrors  int    `json:"input_errors"`
	CRC          int    `json:"crc"`
	OutputErrors int    `json:"output_errors"`
	InputRate    string `json:"input_rate"`
	OutputRate   string `json:"output_rate"`
}

var (
	iosIfStatus = regexp.MustCompile(`(?m)^(\S+) is (administratively down|up|down|deleted), line protocol is (\S+?)(?:\s+\(([^)]+)\))?\s*$`)
	asaIfStatus = regexp.MustCompile(`(?m)^Interface (\S+)(?: "([^"]*)")?, is (administratively down|up|down), line protocol is (\S+)`)
	nxIfStatus  = regexp.MustCompile(`(?m)^(\S+) is (up|down)(?:\s+\(([^)]+)\))?\s*$`)
	ifDesc      = regexp.MustCompile(`(?m)^\s*Description: (.*)$`)
	asaMAC      = regexp.MustCompile(`MAC address ([0-9a-f.]+)`)
	iosDuplex   = regexp.MustCompile(`(?im)^\s*(Full|Half|Auto|a-full|a-half)-duplex, ([^,\n]+)`)
	asaDuplex   = regexp.MustCompile(`(?i)(?:Auto-)?Duplex\(([^)]+)\),\s*(?:Auto-)?Speed\(([^)]+)\)`)
	nxMode      = regexp.MustCompile(`(?m)^\s*Port mode is (\S+)`)
	inErrs      = regexp.MustCompile(`(\d+) input errors?`)
	crcErrs     = regexp.MustCompile(`(\d+) CRC`)
	outErrs     = regexp.MustCompile(`(\d+) output errors?`)
	inRate      = regexp.MustCompile(`(?m)^\s*(\d+ (?:minute|seconds?)) input rate (\d+ bits/sec)`)
	outRate     = regexp.MustCompile(`(?m)^\s*(\d+ (?:minute|seconds?)) output rate (\d+ bits/sec)`)
)

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// ParseInterface handles IOS/IOS-XE, NX-OS and ASA formats.
func ParseInterface(out string) Interface {
	var i Interface
	switch {
	case asaIfStatus.MatchString(out):
		m := asaIfStatus.FindStringSubmatch(out)
		i.Name, i.Description, i.Status, i.Protocol = m[1], m[2], m[3], m[4]
		i.MAC = NormalizeMAC(Find(asaMAC, out))
		if d := asaDuplex.FindStringSubmatch(out); d != nil {
			i.Duplex, i.Speed = d[1], d[2]
		}
	case iosIfStatus.MatchString(out):
		m := iosIfStatus.FindStringSubmatch(out)
		i.Name, i.Status, i.Protocol, i.Reason = m[1], m[2], m[3], m[4]
	case nxIfStatus.MatchString(out):
		m := nxIfStatus.FindStringSubmatch(out)
		i.Name, i.Status, i.Reason = m[1], m[2], m[3]
		i.Protocol = i.Status
	}
	if d := Find(ifDesc, out); d != "" {
		i.Description = d
	}
	if i.MAC == "" {
		i.MAC = InterfaceMAC(out)
	}
	if i.Duplex == "" {
		if d := iosDuplex.FindStringSubmatch(out); d != nil {
			i.Duplex, i.Speed = strings.ToLower(d[1]), strings.TrimSpace(d[2])
		}
	}
	i.Mode = Find(nxMode, out)
	i.InputErrors = atoi(Find(inErrs, out))
	i.CRC = atoi(Find(crcErrs, out))
	i.OutputErrors = atoi(Find(outErrs, out))
	if m := inRate.FindStringSubmatch(out); m != nil {
		i.InputRate = m[2] + " (" + m[1] + ")"
	}
	if m := outRate.FindStringSubmatch(out); m != nil {
		i.OutputRate = m[2] + " (" + m[1] + ")"
	}
	return i
}

// Switchport is the result of `show interfaces X switchport`.
type Switchport struct {
	AdminMode  string `json:"admin_mode"`
	OperMode   string `json:"oper_mode"`
	AccessVlan int    `json:"access_vlan"`
	VlanName   string `json:"vlan_name"`
}

// IsTrunk is true when the port is (or will become) a trunk. Dynamic modes
// are treated as trunks because they can negotiate one.
func (s Switchport) IsTrunk() bool {
	for _, m := range []string{s.AdminMode, s.OperMode} {
		m = strings.ToLower(m)
		if strings.Contains(m, "trunk") || strings.Contains(m, "dynamic") {
			return true
		}
	}
	return false
}

var (
	spAdmin  = regexp.MustCompile(`(?m)^Administrative Mode: (.+)$`)
	spOper   = regexp.MustCompile(`(?m)^Operational Mode: (.+)$`)
	spAccess = regexp.MustCompile(`(?m)^Access Mode VLAN: (\d+)(?: \(([^)]*)\))?`)
)

// ParseSwitchport parses Catalyst `show interfaces X switchport`. ok is
// false when the output does not look like switchport output (e.g. a routed
// port or an unknown interface).
func ParseSwitchport(out string) (sp Switchport, ok bool) {
	sp.AdminMode = Find(spAdmin, out)
	sp.OperMode = Find(spOper, out)
	if m := spAccess.FindStringSubmatch(out); m != nil {
		sp.AccessVlan, sp.VlanName = atoi(m[1]), m[2]
	}
	return sp, sp.AdminMode != ""
}

var vlanRow = regexp.MustCompile(`(?m)^(\d{1,4})\s+(\S+)\s+(active|act/lshut|suspended|act/unsup|sus/lshut)`)

// ParseVlanBrief returns VLAN id -> name (status included) from `show vlan brief`.
func ParseVlanBrief(out string) map[int]VLAN {
	res := map[int]VLAN{}
	for _, m := range vlanRow.FindAllStringSubmatch(out, -1) {
		res[atoi(m[1])] = VLAN{ID: atoi(m[1]), Name: m[2], Status: m[3]}
	}
	return res
}

// VLAN is one row of `show vlan brief`.
type VLAN struct {
	ID     int    `json:"i_d"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// Usable reports whether ports can be assigned to the VLAN.
func (v VLAN) Usable() bool { return v.Status == "active" }
