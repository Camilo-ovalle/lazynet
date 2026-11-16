// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package parse

import (
	"bufio"
	"strings"
	"time"
)

// PANOSInfo is what PAN-OS `show system info` reports.
type PANOSInfo struct {
	Hostname string
	Model    string
	Serial   string
	Version  string // sw-version, e.g. "11.1.4-h7"
	MAC      string // management MAC, as PAN-OS prints it (aa:bb:cc:dd:ee:ff)
	IP       string // management IP
	Uptime   string // e.g. "41 days, 6:12:40"
	Clock    time.Time
}

// PANOSSystemInfo parses `show system info`, a list of "key: value" lines.
func PANOSSystemInfo(out string) PANOSInfo {
	kv := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if _, seen := kv[k]; !seen { // first occurrence wins
			kv[k] = strings.TrimSpace(v)
		}
	}
	unknown := func(s string) string {
		if strings.EqualFold(s, "unknown") {
			return ""
		}
		return s
	}
	info := PANOSInfo{
		Hostname: kv["hostname"],
		Model:    kv["model"],
		Serial:   unknown(kv["serial"]),
		Version:  kv["sw-version"],
		MAC:      unknown(kv["mac-address"]),
		IP:       unknown(kv["ip-address"]),
		Uptime:   kv["uptime"],
	}
	// "Mon Nov 16 22:10:05 2026"; joining fields absorbs the padded day.
	if t, err := time.Parse("Mon Jan 2 15:04:05 2006", strings.Join(strings.Fields(kv["time"]), " ")); err == nil {
		info.Clock = t
	}
	return info
}
