// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package catalyst

import (
	"regexp"
	"strings"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
)

var (
	iosXEVer  = regexp.MustCompile(`(?m)Cisco IOS[ -]XE Software, Version (\S+)`)
	iosVer    = regexp.MustCompile(`(?m)Cisco IOS Software.*?, Version ([^,\s]+)`)
	iosModel  = regexp.MustCompile(`(?im)^Model number\s*:\s*(\S+)`)
	iosModel2 = regexp.MustCompile(`(?im)^cisco (\S+) \(.*\) processor`)
	iosSerial = regexp.MustCompile(`(?im)^System serial number\s*:\s*(\S+)`)
	iosMAC    = regexp.MustCompile(`(?im)^Base ethernet MAC Address\s*:\s*(\S+)`)
	iosUptime = regexp.MustCompile(`(?m)^(\S+) uptime is (.+)$`)
)

// Version parses `show version` of classic IOS and IOS-XE.
func Version(out string) parse.Version {
	v := parse.Version{OS: "IOS"}
	if x := parse.Find(iosXEVer, out); x != "" {
		v.OS, v.Version = "IOS-XE", x
	} else {
		v.Version = parse.Find(iosVer, out)
	}
	v.Model = parse.Find(iosModel, out)
	if v.Model == "" {
		v.Model = parse.Find(iosModel2, out)
	}
	v.Serial = parse.Find(iosSerial, out)
	if v.Serial == "" {
		v.Serial = parse.BoardID(out)
	}
	v.MAC = parse.NormalizeMAC(parse.Find(iosMAC, out))
	if m := iosUptime.FindStringSubmatch(out); m != nil {
		v.Hostname, v.Uptime = m[1], strings.TrimSpace(m[2])
	}
	return v
}
