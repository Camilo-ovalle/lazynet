// Package parse turns Cisco CLI output into structured data. Parsers are
// lenient: they extract what they recognise and leave fields empty otherwise,
// so callers can always fall back to showing the raw output.
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

func find(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

var (
	iosXEVer     = regexp.MustCompile(`(?m)Cisco IOS[ -]XE Software, Version (\S+)`)
	iosVer       = regexp.MustCompile(`(?m)Cisco IOS Software.*?, Version ([^,\s]+)`)
	iosModel     = regexp.MustCompile(`(?im)^Model number\s*:\s*(\S+)`)
	iosModel2    = regexp.MustCompile(`(?im)^cisco (\S+) \(.*\) processor`)
	iosSerial    = regexp.MustCompile(`(?im)^System serial number\s*:\s*(\S+)`)
	boardID      = regexp.MustCompile(`(?im)Processor board ID (\S+)`)
	iosMAC       = regexp.MustCompile(`(?im)^Base ethernet MAC Address\s*:\s*(\S+)`)
	iosUptime    = regexp.MustCompile(`(?m)^(\S+) uptime is (.+)$`)
	nxVer        = regexp.MustCompile(`(?im)^\s*(?:NXOS|system):\s+version (\S+)`)
	nxModel      = regexp.MustCompile(`(?im)^\s*cisco (Nexus.*?) [Cc]hassis`)
	nxHost       = regexp.MustCompile(`(?im)^\s*Device name:\s*(\S+)`)
	nxUptime     = regexp.MustCompile(`(?im)^Kernel uptime is (.+)$`)
	asaVer       = regexp.MustCompile(`(?m)Adaptive Security Appliance Software Version (\S+)`)
	asaModel     = regexp.MustCompile(`(?m)^Hardware:\s+([^,]+),`)
	asaSerial    = regexp.MustCompile(`(?m)^Serial Number:\s*(\S+)`)
	asaUptime    = regexp.MustCompile(`(?m)^(\S+) up (.+)$`)
	asaMgmtMAC   = regexp.MustCompile(`(?m)Management\S*\s*: address is (\S+),`)
	asaFirstMAC  = regexp.MustCompile(`(?m)Ext: \S+\s*: address is (\S+),`)
	invSN        = regexp.MustCompile(`SN:\s*(\S+)`)
	ifaceMAC     = regexp.MustCompile(`(?i)address(?: is|:) ([0-9a-f]{4}\.[0-9a-f]{4}\.[0-9a-f]{4})`)
	nxMgmtIP     = regexp.MustCompile(`(?im)Internet Address is (\S+)`)
	macSeparator = regexp.MustCompile(`[^0-9a-f]`)
)

// IOSVersion parses Catalyst `show version` (classic IOS and IOS-XE).
func IOSVersion(out string) Version {
	v := Version{OS: "IOS"}
	if x := find(iosXEVer, out); x != "" {
		v.OS, v.Version = "IOS-XE", x
	} else {
		v.Version = find(iosVer, out)
	}
	v.Model = find(iosModel, out)
	if v.Model == "" {
		v.Model = find(iosModel2, out)
	}
	v.Serial = find(iosSerial, out)
	if v.Serial == "" {
		v.Serial = find(boardID, out)
	}
	v.MAC = NormalizeMAC(find(iosMAC, out))
	if m := iosUptime.FindStringSubmatch(out); m != nil {
		v.Hostname, v.Uptime = m[1], strings.TrimSpace(m[2])
	}
	return v
}

// NXOSVersion parses Nexus `show version`.
func NXOSVersion(out string) Version {
	return Version{
		OS:       "NX-OS",
		Version:  find(nxVer, out),
		Model:    find(nxModel, out),
		Serial:   find(boardID, out),
		Hostname: find(nxHost, out),
		Uptime:   find(nxUptime, out),
	}
}

// ASAVersion parses ASA `show version`.
func ASAVersion(out string) Version {
	v := Version{OS: "ASA", Version: find(asaVer, out), Model: find(asaModel, out), Serial: find(asaSerial, out)}
	for _, m := range asaUptime.FindAllStringSubmatch(out, -1) {
		if m[1] != "failover" {
			v.Hostname, v.Uptime = m[1], strings.TrimSpace(m[2])
			break
		}
	}
	v.MAC = find(asaMgmtMAC, out)
	if v.MAC == "" {
		v.MAC = find(asaFirstMAC, out)
	}
	v.MAC = NormalizeMAC(v.MAC)
	return v
}

// InventorySerial returns the first serial number in `show inventory`
// (the chassis / stack on every platform).
func InventorySerial(out string) string { return find(invSN, out) }

// InterfaceMAC extracts the burned-in address from `show interface X`.
func InterfaceMAC(out string) string { return NormalizeMAC(find(ifaceMAC, out)) }

// NXOSMgmtIP extracts the address of `show interface mgmt0`.
func NXOSMgmtIP(out string) string { return find(nxMgmtIP, out) }

// NormalizeMAC renders any MAC notation in Cisco dotted form (aaaa.bbbb.cccc).
func NormalizeMAC(s string) string {
	h := macSeparator.ReplaceAllString(strings.ToLower(s), "")
	if len(h) != 12 {
		return s
	}
	return h[0:4] + "." + h[4:8] + "." + h[8:12]
}
