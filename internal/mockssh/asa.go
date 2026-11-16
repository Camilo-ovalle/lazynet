// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package mockssh

import (
	"fmt"
	"strings"
	"time"
)

// ASA: named interfaces, no switchports, "terminal pager" instead of length.
func init() {
	register("asa", &ios{
		init:     asaInit,
		logs:     asaLogs,
		iface:    asaIface,
		terminal: rejectTerminal("length", "ERROR: "+invalid),
	})
}

func asaInit(d *Device) {
	for i, n := range []string{"outside", "inside", ""} {
		d.ports = append(d.ports, &Port{Name: fmt.Sprintf("GigabitEthernet1/%d", i+1), Short: fmt.Sprintf("Gi1/%d", i+1), Up: n != "", Desc: n, MAC: fmt.Sprintf("00a3.8e4b.1c%02x", i+1)})
	}
	d.ports = append(d.ports, &Port{Name: "Management1/1", Short: "Ma1/1", Up: true, Desc: "management", MAC: "00a3.8e4b.1c00"})
}

var asaLogLines = []logLine{
	{50 * time.Hour, "%ASA-5-111008: User 'enable_15' executed the 'write memory' command."},
	{30 * time.Hour, "%ASA-4-106023: Deny tcp src outside:198.51.100.7/51234 dst inside:192.168.100.10/3389 by access-group \"OUTSIDE_IN\" [0x0, 0x0]"},
	{10 * time.Hour, "%ASA-6-605005: Login permitted from 192.168.100.50/50122 to inside:192.168.100.1/ssh for user \"fwadmin\""},
	{4 * time.Hour, "%ASA-3-710003: TCP access denied by ACL from 198.51.100.9/41000 to outside:203.0.113.2/22"},
	{1 * time.Hour, "%ASA-6-302013: Built inbound TCP connection 1234 for outside:198.51.100.20/443 (198.51.100.20/443) to inside:192.168.100.25/55312 (203.0.113.2/55312)"},
}

func asaLogs(_ *Device, now time.Time) string {
	var b strings.Builder
	now = now.UTC()
	b.WriteString("Syslog logging: enabled\n    Timestamp logging: enabled\n    Buffer logging: level informational, 2310 messages logged\n")
	for _, l := range asaLogLines {
		fmt.Fprintf(&b, "%s: %s\n", now.Add(-l.ago).Format("Jan 02 2006 15:04:05"), l.msg)
	}
	return strings.TrimRight(b.String(), "\n")
}

func asaIface(_ *Device, p *Port) string {
	ps := stateOf(p)
	nameif := ""
	if p.Desc != "" {
		nameif = fmt.Sprintf(" %q,", p.Desc)
	}
	return fmt.Sprintf(`Interface %s%s is %s, line protocol is %s
  Hardware is Accelerator rev01, BW 1000 Mbps, DLY 10 usec
	Auto-Duplex(Full-duplex), Auto-Speed(1000 Mbps)
	Input flow control is unsupported, output flow control is off
	MAC address %s, MTU 1500
	1234567 packets input, 987654321 bytes, 0 no buffer
	Received 1234 broadcasts, 0 runts, 0 giants
	%d input errors, %d CRC, 0 frame, 0 overrun, 0 ignored, 0 abort
	2345678 packets output, 1234567890 bytes, 0 underruns
	0 output errors, 0 collisions, 0 interface resets`, p.Name, nameif, ps.st, ps.proto, p.MAC, ps.crc, ps.crc)
}
