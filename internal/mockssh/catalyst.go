// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package mockssh

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Catalyst (IOS-XE): 48 access ports, two trunk uplinks and a few VLANs, so
// the VLAN-change workflow can run end to end.
func init() {
	register("catalyst", &ios{
		init:       catalystInit,
		logs:       catalystLogs,
		switchport: true,
		vlanBrief:  true,
		terminal:   rejectTerminal("pager", invalid),
	})
}

func catalystInit(d *Device) {
	d.vlans = map[int]string{1: "default", 10: "DATA", 20: "VOICE", 110: "PRINTERS", 999: "PARKING"}
	descs := map[int]string{1: "PC-Office-12", 2: "PC-Office-13", 7: "Printer-2F", 12: "AP-Lobby", 25: "Camera-Parking"}
	for i := 1; i <= 48; i++ {
		v := 10
		switch {
		case i == 7 || i == 8:
			v = 110
		case i >= 40:
			v = 999
		}
		d.ports = append(d.ports, &Port{
			Name: fmt.Sprintf("GigabitEthernet1/0/%d", i), Short: fmt.Sprintf("Gi1/0/%d", i),
			Up: i%5 != 0 && i < 40, Desc: descs[i], Vlan: v, MAC: fmt.Sprintf("706d.15a2.3b%02x", 0x80+i),
		})
	}
	for i := 1; i <= 2; i++ {
		d.ports = append(d.ports, &Port{
			Name: fmt.Sprintf("TenGigabitEthernet1/1/%d", i), Short: fmt.Sprintf("Te1/1/%d", i),
			Up: true, Desc: fmt.Sprintf("uplink-core-%d", i), Trunk: true, Vlan: 1, MAC: fmt.Sprintf("706d.15a2.3c%02x", i),
		})
	}
}

var catalystLogLines = []logLine{
	{70 * time.Hour, "%SYS-5-CONFIG_I: Configured from console by admin on vty0 (192.168.100.50)"},
	{52 * time.Hour, "%LINK-3-UPDOWN: Interface GigabitEthernet1/0/12, changed state to down"},
	{52*time.Hour - 3*time.Second, "%LINEPROTO-5-UPDOWN: Line protocol on Interface GigabitEthernet1/0/12, changed state to down"},
	{51 * time.Hour, "%LINK-3-UPDOWN: Interface GigabitEthernet1/0/12, changed state to up"},
	{30 * time.Hour, "%SEC_LOGIN-5-LOGIN_SUCCESS: Login Success [user: admin] [Source: 192.168.100.50] [localport: 22]"},
	{26 * time.Hour, "%PM-4-ERR_DISABLE: bpduguard error detected on Gi1/0/33, putting Gi1/0/33 in err-disable state"},
	{20 * time.Hour, "%ILPOWER-5-POWER_GRANTED: Interface Gi1/0/12: Power granted"},
	{9 * time.Hour, "%SW_MATM-4-MACFLAP_NOTIF: Host 0050.56a1.2b3c in vlan 10 is flapping between port Gi1/0/3 and port Gi1/0/4"},
	{5 * time.Hour, "%SEC_LOGIN-4-LOGIN_FAILED: Login failed [user: root] [Source: 10.0.0.66] [localport: 22] [Reason: Login Authentication Failed]"},
	{2 * time.Hour, "%LINK-3-UPDOWN: Interface GigabitEthernet1/0/25, changed state to down"},
	{90 * time.Minute, "%LINK-3-UPDOWN: Interface GigabitEthernet1/0/25, changed state to up"},
	{20 * time.Minute, "%SYS-6-LOGGINGHOST_STARTSTOP: Logging to host 192.168.100.5 port 514 started - CLI initiated"},
}

func catalystLogs(_ *Device, now time.Time) string {
	var b strings.Builder
	now = now.UTC()
	b.WriteString("Syslog logging: enabled (0 messages dropped, 3 messages rate-limited, 0 flushes, 0 overruns, xml disabled, filtering disabled)\n\n")
	b.WriteString("    Buffer logging:  level debugging, 1532 messages logged, xml disabled,\n                    filtering disabled\n\nLog Buffer (64000 bytes):\n\n")
	for _, l := range catalystLogLines {
		fmt.Fprintf(&b, "%s: %s\n", now.Add(-l.ago).Format("Jan _2 2006 15:04:05.000"), l.msg)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (d *Device) vlanBrief() string {
	ids := make([]int, 0, len(d.vlans))
	for id := range d.vlans {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	var b strings.Builder
	b.WriteString("\nVLAN Name                             Status    Ports\n")
	b.WriteString("---- -------------------------------- --------- -------------------------------\n")
	for _, id := range ids {
		var ports []string
		for _, p := range d.ports {
			if !p.Trunk && p.Vlan == id {
				ports = append(ports, p.Short)
			}
		}
		first := true
		for len(ports) > 0 || first {
			n := min(4, len(ports))
			chunk := strings.Join(ports[:n], ", ")
			ports = ports[n:]
			if first {
				fmt.Fprintf(&b, "%-4d %-32s %-9s %s\n", id, d.vlans[id], "active", chunk)
				first = false
			} else {
				fmt.Fprintf(&b, "%-4s %-32s %-9s %s\n", "", "", "", chunk)
			}
		}
	}
	for _, x := range []string{"1002 fddi-default                     act/unsup", "1003 token-ring-default               act/unsup", "1004 fddinet-default                  act/unsup", "1005 trnet-default                    act/unsup"} {
		b.WriteString(x + " \n")
	}
	return b.String()
}
