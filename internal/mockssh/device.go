// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package mockssh is a fake Cisco SSH server used by tests and demo mode.
// It emulates prompts, privilege/config modes, output filters and a small
// amount of state (switchport VLANs) so the full VLAN-change workflow can be
// exercised without touching a real device.
package mockssh

import (
	"embed"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed fixtures
var fixtures embed.FS

// Port is an emulated interface.
type Port struct {
	Name  string // canonical, e.g. GigabitEthernet1/0/1
	Short string // e.g. Gi1/0/1
	Up    bool
	Desc  string
	Trunk bool
	Vlan  int
	MAC   string
}

// Device is the emulated device state.
type Device struct {
	Hostname     string
	Platform     string // catalyst | nexus | asa
	Unprivileged bool   // log in at '>' instead of '#'
	Now          func() time.Time

	mu       sync.Mutex
	ports    []*Port
	vlans    map[int]string
	commands []string
	saved    bool
	static   map[string]string
}

// NewDevice builds a device with realistic default state for the platform.
func NewDevice(platform, hostname string) *Device {
	d := &Device{Hostname: hostname, Platform: platform, Now: time.Now, vlans: map[int]string{}, static: map[string]string{}}
	entries, _ := fixtures.ReadDir("fixtures/" + platform)
	for _, e := range entries {
		b, _ := fixtures.ReadFile("fixtures/" + platform + "/" + e.Name())
		cmd := strings.ReplaceAll(strings.TrimSuffix(e.Name(), ".txt"), "_", " ")
		d.static[cmd] = string(b)
	}
	switch platform {
	case "catalyst":
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
	case "nexus":
		for i := 1; i <= 8; i++ {
			p := &Port{Name: fmt.Sprintf("Ethernet1/%d", i), Short: fmt.Sprintf("Eth1/%d", i), Up: i <= 6, Vlan: 10, MAC: fmt.Sprintf("2c4f.52a1.b0%02x", 0xd0+i)}
			if i == 1 {
				p.Trunk, p.Desc = true, "uplink-core"
			}
			d.ports = append(d.ports, p)
		}
	case "asa":
		for i, n := range []string{"outside", "inside", ""} {
			d.ports = append(d.ports, &Port{Name: fmt.Sprintf("GigabitEthernet1/%d", i+1), Short: fmt.Sprintf("Gi1/%d", i+1), Up: n != "", Desc: n, MAC: fmt.Sprintf("00a3.8e4b.1c%02x", i+1)})
		}
		d.ports = append(d.ports, &Port{Name: "Management1/1", Short: "Ma1/1", Up: true, Desc: "management", MAC: "00a3.8e4b.1c00"})
	}
	return d
}

// Commands returns every command line received so far (for assertions).
func (d *Device) Commands() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.commands...)
}

// Saved reports whether "write memory" was executed.
func (d *Device) Saved() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.saved
}

// PortVlan returns the access VLAN of a port (canonical name).
func (d *Device) PortVlan(name string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	if p := d.port(name); p != nil {
		return p.Vlan
	}
	return -1
}

func (d *Device) port(name string) *Port {
	n := strings.ToLower(name)
	for _, p := range d.ports {
		if strings.ToLower(p.Name) == n || strings.ToLower(p.Short) == n {
			return p
		}
	}
	return nil
}

const invalid = "% Invalid input detected at '^' marker."

type mode int

const (
	modeExec mode = iota
	modePriv
	modeConfig
	modeConfigIf
)

// session is per-connection CLI state.
type session struct {
	d     *Device
	mode  mode
	iface *Port
}

func (s *session) prompt() string {
	switch s.mode {
	case modeExec:
		return s.d.Hostname + ">"
	case modeConfig:
		return s.d.Hostname + "(config)#"
	case modeConfigIf:
		return s.d.Hostname + "(config-if)#"
	}
	return s.d.Hostname + "#"
}

// handle executes one line and returns output and whether to close.
func (s *session) handle(line string) (string, bool) {
	line = strings.TrimSpace(line)
	d := s.d
	d.mu.Lock()
	defer d.mu.Unlock()
	if line == "" {
		return "", false
	}
	d.commands = append(d.commands, line)
	f := strings.Fields(line)
	switch s.mode {
	case modeConfig, modeConfigIf:
		return s.config(f), false
	case modeExec:
		if line == "exit" {
			return "", true
		}
		if strings.HasPrefix(line, "show") {
			return s.show(line), false
		}
		return invalid, false
	}
	switch {
	case line == "exit" || line == "logout":
		return "", true
	case line == "configure terminal" || line == "conf t":
		s.mode = modeConfig
		return "Enter configuration commands, one per line.  End with CNTL/Z.", false
	case line == "write memory" || line == "copy running-config startup-config":
		d.saved = true
		if d.Platform == "nexus" {
			return "[########################################] 100%\nCopy complete, now saving to disk (please wait)...\nCopy complete.", false
		}
		return "Building configuration...\n[OK]", false
	case f[0] == "terminal":
		if d.Platform == "asa" && len(f) > 1 && f[1] == "length" {
			return "ERROR: " + invalid, false
		}
		if d.Platform != "asa" && len(f) > 1 && f[1] == "pager" {
			return invalid, false
		}
		return "", false
	case f[0] == "show":
		return s.show(line), false
	}
	return invalid, false
}

func (s *session) config(f []string) string {
	d := s.d
	switch {
	case f[0] == "end":
		s.mode, s.iface = modePriv, nil
		return ""
	case f[0] == "exit":
		if s.mode == modeConfigIf {
			s.mode, s.iface = modeConfig, nil
		} else {
			s.mode = modePriv
		}
		return ""
	case f[0] == "interface" && len(f) >= 2:
		p := d.port(strings.Join(f[1:], ""))
		if p == nil {
			return invalid
		}
		s.mode, s.iface = modeConfigIf, p
		return ""
	case s.mode == modeConfigIf && len(f) == 4 && f[0] == "switchport" && f[1] == "access" && f[2] == "vlan":
		v, err := strconv.Atoi(f[3])
		if err != nil || v < 1 || v > 4094 {
			return invalid
		}
		s.iface.Vlan = v
		if _, ok := d.vlans[v]; !ok {
			d.vlans[v] = fmt.Sprintf("VLAN%04d", v)
			return "% Access VLAN does not exist. Creating vlan " + f[3]
		}
		return ""
	}
	return "" // accept other config lines silently
}

var pipeRe = regexp.MustCompile(`^(.*?)\s*\|\s*(include|exclude|begin|inc|i|ex|e|b)\s+(.+)$`)

func (s *session) show(line string) string {
	base, filter, expr := line, "", ""
	if m := pipeRe.FindStringSubmatch(line); m != nil {
		base, filter, expr = m[1], m[2], m[3]
	}
	out := s.showBase(strings.Join(strings.Fields(base), " "))
	if filter == "" {
		return out
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return invalid
	}
	var keep []string
	begun := false
	for _, l := range strings.Split(out, "\n") {
		switch filter[0] {
		case 'i':
			if re.MatchString(l) {
				keep = append(keep, l)
			}
		case 'e':
			if !re.MatchString(l) {
				keep = append(keep, l)
			}
		case 'b':
			begun = begun || re.MatchString(l)
			if begun {
				keep = append(keep, l)
			}
		}
	}
	return strings.Join(keep, "\n")
}

func (s *session) showBase(cmd string) string {
	d := s.d
	now := d.Now()
	lc := strings.ToLower(cmd)
	switch {
	case lc == "show clock":
		c := now.UTC().Format("15:04:05.000 MST Mon Jan 2 2006")
		if d.Platform == "nexus" {
			return c + "\nTime source is NTP"
		}
		return c
	case lc == "show logging" || lc == "show logging logfile":
		return d.logs(now)
	case lc == "show vlan brief" && d.Platform == "catalyst":
		return d.vlanBrief()
	case lc == "show running-config":
		return d.runningConfig(now)
	case strings.HasPrefix(lc, "show interfaces ") || strings.HasPrefix(lc, "show interface "):
		f := strings.Fields(cmd)
		if len(f) < 3 {
			return invalid
		}
		name := f[2]
		if strings.EqualFold(name, "mgmt0") && d.Platform == "nexus" {
			return d.render(d.static["show interface mgmt0"], now)
		}
		p := d.port(name)
		if p == nil {
			return invalid
		}
		if len(f) == 4 && f[3] == "switchport" {
			if d.Platform == "asa" {
				return invalid
			}
			return d.switchport(p)
		}
		return d.iface(p)
	}
	if out, ok := d.static[lc]; ok {
		return d.render(out, now)
	}
	return invalid
}

func (d *Device) render(s string, now time.Time) string {
	s = strings.ReplaceAll(s, "{{HOST}}", d.Hostname)
	s = strings.ReplaceAll(s, "{{NOW}}", now.UTC().Format("Mon Jan  2 15:04:05 2006"))
	return strings.TrimRight(s, "\n")
}

func (d *Device) runningConfig(now time.Time) string {
	var b strings.Builder
	for _, p := range d.ports {
		fmt.Fprintf(&b, "interface %s\n", p.Name)
		if p.Desc != "" {
			fmt.Fprintf(&b, " description %s\n", p.Desc)
		}
		if p.Trunk {
			b.WriteString(" switchport mode trunk\n")
		} else {
			fmt.Fprintf(&b, " switchport access vlan %d\n switchport mode access\n", p.Vlan)
		}
		b.WriteString("!\n")
	}
	return strings.ReplaceAll(d.render(d.static["show running-config"], now), "{{INTERFACES}}", b.String())
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

func (d *Device) switchport(p *Port) string {
	adm, op := "static access", "static access"
	if p.Trunk {
		adm, op = "trunk", "trunk"
	}
	if !p.Up && !p.Trunk {
		op = "down"
	}
	name := d.vlans[p.Vlan]
	if name == "" {
		name = "Inactive"
	}
	return fmt.Sprintf(`Name: %s
Switchport: Enabled
Administrative Mode: %s
Operational Mode: %s
Administrative Trunking Encapsulation: dot1q
Operational Trunking Encapsulation: native
Negotiation of Trunking: Off
Access Mode VLAN: %d (%s)
Trunking Native Mode VLAN: 1 (default)
Administrative Native VLAN tagging: enabled
Voice VLAN: none
Administrative private-vlan host-association: none
Trunking VLANs Enabled: ALL
Pruning VLANs Enabled: 2-1001
Capture Mode Disabled
Capture VLANs Allowed: ALL

Protected: false
Unknown unicast blocked: disabled
Unknown multicast blocked: disabled
Appliance trust: none`, p.Short, adm, op, p.Vlan, name)
}

func (d *Device) iface(p *Port) string {
	st, proto, tag := "up", "up", "connected"
	duplex := "Full-duplex, 1000Mb/s, media type is 10/100/1000BaseTX"
	if !p.Up {
		st, proto, tag = "down", "down", "notconnect"
		duplex = "Auto-duplex, Auto-speed, media type is 10/100/1000BaseTX"
	}
	if strings.HasPrefix(p.Name, "TenGig") {
		duplex = "Full-duplex, 10Gb/s, link type is auto, media type is SFP-10GBase-SR"
	}
	desc := ""
	if p.Desc != "" {
		desc = "\n  Description: " + p.Desc
	}
	crc := len(p.Name) % 3
	switch d.Platform {
	case "nexus":
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
    0 output error  0 collision  0 deferred  0 late collision`, p.Name, st, p.MAC, p.MAC, desc, mode, crc, crc)
	case "asa":
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
	0 output errors, 0 collisions, 0 interface resets`, p.Name, nameif, st, proto, p.MAC, crc, crc)
	}
	return fmt.Sprintf(`%s is %s, line protocol is %s (%s) 
  Hardware is Gigabit Ethernet, address is %s (bia %s)%s
  MTU 1500 bytes, BW 1000000 Kbit/sec, DLY 10 usec, 
     reliability 255/255, txload 1/255, rxload 1/255
  Encapsulation ARPA, loopback not set
  Keepalive set (10 sec)
  %s
  input flow-control is on, output flow-control is unsupported 
  ARP type: ARPA, ARP Timeout 04:00:00
  Last input 00:00:01, output 00:00:00, output hang never
  Last clearing of "show interface" counters never
  Input queue: 0/2000/0/0 (size/max/drops/flushes); Total output drops: 0
  Queueing strategy: fifo
  Output queue: 0/40 (size/max)
  5 minute input rate 12000 bits/sec, 15 packets/sec
  5 minute output rate 45000 bits/sec, 30 packets/sec
     8123456 packets input, 1234567890 bytes, 0 no buffer
     Received 123456 broadcasts (98765 multicasts)
     0 runts, 0 giants, 0 throttles 
     %d input errors, %d CRC, 0 frame, 0 overrun, 0 ignored
     0 watchdog, 98765 multicast, 0 pause input
     0 input packets with dribble condition detected
     9123456 packets output, 2345678901 bytes, 0 underruns
     0 output errors, 0 collisions, 1 interface resets
     0 unknown protocol drops
     0 babbles, 0 late collision, 0 deferred
     0 lost carrier, 0 no carrier, 0 pause output
     0 output buffer failures, 0 output buffers swapped out`, p.Name, st, proto, tag, p.MAC, p.MAC, desc, duplex, crc, crc)
}

type logLine struct {
	ago time.Duration
	msg string
}

var catalystLogs = []logLine{
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

var nexusLogs = []logLine{
	{60 * time.Hour, "%VSHD-5-VSHD_SYSLOG_CONFIG_I: Configured from vty by admin on 192.168.100.50@pts/0"},
	{40 * time.Hour, "%ETHPORT-5-IF_DOWN_LINK_FAILURE: Interface Ethernet1/7 is down (Link failure)"},
	{39 * time.Hour, "%ETHPORT-5-IF_UP: Interface Ethernet1/7 is up in mode access"},
	{12 * time.Hour, "%VPC-2-PEER_KEEP_ALIVE_RECV_FAIL: In domain 10, VPC peer keep-alive receive has failed"},
	{11 * time.Hour, "%VPC-2-PEER_KEEP_ALIVE_RECV_SUCCESS: In domain 10, vPC peer keep-alive receive is successful"},
	{3 * time.Hour, "%AUTHPRIV-6-SYSTEM_MSG: pam_unix(aaa:session): session opened for user admin - dcos_sshd"},
}

var asaLogs = []logLine{
	{50 * time.Hour, "%ASA-5-111008: User 'enable_15' executed the 'write memory' command."},
	{30 * time.Hour, "%ASA-4-106023: Deny tcp src outside:198.51.100.7/51234 dst inside:192.168.100.10/3389 by access-group \"OUTSIDE_IN\" [0x0, 0x0]"},
	{10 * time.Hour, "%ASA-6-605005: Login permitted from 192.168.100.50/50122 to inside:192.168.100.1/ssh for user \"fwadmin\""},
	{4 * time.Hour, "%ASA-3-710003: TCP access denied by ACL from 198.51.100.9/41000 to outside:203.0.113.2/22"},
	{1 * time.Hour, "%ASA-6-302013: Built inbound TCP connection 1234 for outside:198.51.100.20/443 (198.51.100.20/443) to inside:192.168.100.25/55312 (203.0.113.2/55312)"},
}

func (d *Device) logs(now time.Time) string {
	var b strings.Builder
	now = now.UTC()
	switch d.Platform {
	case "nexus":
		for _, l := range nexusLogs {
			fmt.Fprintf(&b, "%s %s %s\n", now.Add(-l.ago).Format("2006 Jan _2 15:04:05.000"), d.Hostname, l.msg)
		}
	case "asa":
		b.WriteString("Syslog logging: enabled\n    Timestamp logging: enabled\n    Buffer logging: level informational, 2310 messages logged\n")
		for _, l := range asaLogs {
			fmt.Fprintf(&b, "%s: %s\n", now.Add(-l.ago).Format("Jan 02 2006 15:04:05"), l.msg)
		}
	default:
		b.WriteString("Syslog logging: enabled (0 messages dropped, 3 messages rate-limited, 0 flushes, 0 overruns, xml disabled, filtering disabled)\n\n")
		b.WriteString("    Buffer logging:  level debugging, 1532 messages logged, xml disabled,\n                    filtering disabled\n\nLog Buffer (64000 bytes):\n\n")
		for _, l := range catalystLogs {
			fmt.Fprintf(&b, "%s: %s\n", now.Add(-l.ago).Format("Jan _2 2006 15:04:05.000"), l.msg)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// Exec runs one command in a fresh privileged session and returns its output
// (used by parser tests to reuse the emulator's fixtures).
func (d *Device) Exec(cmd string) string {
	s := &session{d: d, mode: modePriv}
	out, _ := s.handle(cmd)
	return out
}
