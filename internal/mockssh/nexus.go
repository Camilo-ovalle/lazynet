// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package mockssh

import (
	"fmt"
	"strings"
	"time"
)

// Nexus (NX-OS): eight Ethernet ports, mgmt0 from a fixture, logfile format.
func init() {
	register("nexus", &ios{
		init:        nexusInit,
		clockSuffix: "\nTime source is NTP",
		logs:        nexusLogs,
		iface:       nexusIface,
		switchport:  true,
		terminal:    rejectTerminal("pager", invalid),
		saveOutput:  "[########################################] 100%\nCopy complete, now saving to disk (please wait)...\nCopy complete.",
	})
}

func nexusInit(d *Device) {
	for i := 1; i <= 8; i++ {
		p := &Port{Name: fmt.Sprintf("Ethernet1/%d", i), Short: fmt.Sprintf("Eth1/%d", i), Up: i <= 6, Vlan: 10, MAC: fmt.Sprintf("2c4f.52a1.b0%02x", 0xd0+i)}
		if i == 1 {
			p.Trunk, p.Desc = true, "uplink-core"
		}
		d.ports = append(d.ports, p)
	}
}

var nexusLogLines = []logLine{
	{60 * time.Hour, "%VSHD-5-VSHD_SYSLOG_CONFIG_I: Configured from vty by admin on 192.168.100.50@pts/0"},
	{40 * time.Hour, "%ETHPORT-5-IF_DOWN_LINK_FAILURE: Interface Ethernet1/7 is down (Link failure)"},
	{39 * time.Hour, "%ETHPORT-5-IF_UP: Interface Ethernet1/7 is up in mode access"},
	{12 * time.Hour, "%VPC-2-PEER_KEEP_ALIVE_RECV_FAIL: In domain 10, VPC peer keep-alive receive has failed"},
	{11 * time.Hour, "%VPC-2-PEER_KEEP_ALIVE_RECV_SUCCESS: In domain 10, vPC peer keep-alive receive is successful"},
	{3 * time.Hour, "%AUTHPRIV-6-SYSTEM_MSG: pam_unix(aaa:session): session opened for user admin - dcos_sshd"},
}

func nexusLogs(d *Device, now time.Time) string {
	var b strings.Builder
	now = now.UTC()
	for _, l := range nexusLogLines {
		fmt.Fprintf(&b, "%s %s %s\n", now.Add(-l.ago).Format("2006 Jan _2 15:04:05.000"), d.Hostname, l.msg)
	}
	return strings.TrimRight(b.String(), "\n")
}

func nexusIface(_ *Device, p *Port) string {
	ps := stateOf(p)
	mode := "access"
	if p.Trunk {
		mode = "trunk"
	}
	return fmt.Sprintf(`%s is %s
admin state is up, Dedicated Interface
  Hardware: 100/1000/10000/25000 Ethernet, address: %s (bia %s)%s
  MTU 1500 bytes, BW 10000000 Kbit , DLY 10 usec
  reliability 255/255, txload 1/255, rxload 1/255
  Encapsulation ARPA, medium is broadcast
  Port mode is %s
  full-duplex, 10 Gb/s, media type is 10G
  Beacon is turned off
  Last link flapped 2week(s) 3day(s)
  30 seconds input rate 123456 bits/sec, 50 packets/sec
  30 seconds output rate 654321 bits/sec, 80 packets/sec
  RX
    120394 unicast packets  4321 multicast packets  12 broadcast packets
    124727 input packets  98234123 bytes
    0 runts  0 giants  %d CRC  0 no buffer
    %d input error  0 short frame  0 overrun   0 underrun  0 ignored
  TX
    230111 unicast packets  1234 multicast packets  10 broadcast packets
    231355 output packets  182736455 bytes
    0 output error  0 collision  0 deferred  0 late collision`, p.Name, ps.st, p.MAC, p.MAC, ps.desc, mode, ps.crc, ps.crc)
}
