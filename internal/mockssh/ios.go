// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package mockssh

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const invalid = "% Invalid input detected at '^' marker."

// ios is the shared engine of Cisco-style CLIs (IOS, IOS-XE, NX-OS, ASA):
// user/privileged/config modes, "| include/exclude/begin" filters, stateful
// ports and VLANs. Each platform file configures one with its own state,
// outputs and quirks.
type ios struct {
	// init builds the default ports and VLANs.
	init func(d *Device)
	// clockSuffix is appended to "show clock" (NX-OS adds the time source).
	clockSuffix string
	// logs renders the log buffer ("show logging" / "show logging logfile").
	logs func(d *Device, now time.Time) string
	// iface renders "show interface(s) X"; nil uses the IOS format.
	iface func(d *Device, p *Port) string
	// switchport enables "show interfaces X switchport"; vlanBrief enables
	// "show vlan brief".
	switchport, vlanBrief bool
	// terminal answers "terminal ..." settings: "" accepts them.
	terminal func(args []string) string
	// saveOutput answers "write memory" / "copy running-config startup-config".
	saveOutput string
}

func (e *ios) setup(d *Device) {
	if e.init != nil {
		e.init(d)
	}
}

func (e *ios) prompt(s *session) string {
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

func (e *ios) exec(s *session, line string) (string, bool) {
	d := s.d
	f := strings.Fields(line)
	switch s.mode {
	case modeConfig, modeConfigIf:
		return e.config(s, f), false
	case modeExec:
		if line == "exit" {
			return "", true
		}
		if strings.HasPrefix(line, "show") {
			return e.show(s, line), false
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
		if e.saveOutput == "" {
			return "Building configuration...\n[OK]", false
		}
		return e.saveOutput, false
	case f[0] == "terminal":
		if e.terminal != nil {
			return e.terminal(f[1:]), false
		}
		return "", false
	case f[0] == "show":
		return e.show(s, line), false
	}
	return invalid, false
}

func (e *ios) config(s *session, f []string) string {
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

func (e *ios) show(s *session, line string) string {
	base, filter, expr := line, "", ""
	if m := pipeRe.FindStringSubmatch(line); m != nil {
		base, filter, expr = m[1], m[2], m[3]
	}
	out := e.showBase(s, strings.Join(strings.Fields(base), " "))
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

func (e *ios) showBase(s *session, cmd string) string {
	d := s.d
	now := d.Now()
	lc := strings.ToLower(cmd)
	switch {
	case lc == "show clock":
		return now.UTC().Format("15:04:05.000 MST Mon Jan 2 2006") + e.clockSuffix
	case (lc == "show logging" || lc == "show logging logfile") && e.logs != nil:
		return e.logs(d, now)
	case lc == "show vlan brief" && e.vlanBrief:
		return d.vlanBrief()
	case lc == "show running-config":
		return d.runningConfig(now)
	case strings.HasPrefix(lc, "show interfaces ") || strings.HasPrefix(lc, "show interface "):
		if out, ok := d.static[lc]; ok { // e.g. NX-OS "show interface mgmt0"
			return d.render(out, now)
		}
		f := strings.Fields(cmd)
		if len(f) < 3 {
			return invalid
		}
		p := d.port(f[2])
		if p == nil {
			return invalid
		}
		if len(f) == 4 && f[3] == "switchport" {
			if !e.switchport {
				return invalid
			}
			return d.switchport(p)
		}
		if e.iface != nil {
			return e.iface(d, p)
		}
		return iosIface(p)
	}
	if out, ok := d.static[lc]; ok {
		return d.render(out, now)
	}
	return invalid
}

// rejectTerminal refuses "terminal <setting>" the way the platform does.
func rejectTerminal(setting, answer string) func([]string) string {
	return func(args []string) string {
		if len(args) > 0 && args[0] == setting {
			return answer
		}
		return ""
	}
}

func (d *Device) runningConfig(now time.Time) string {
	var b strings.Builder
	for _, p := range d.ports {
		fmt.Fprintf(&b, "interface %s\n", p.Name)
		if p.Desc != "" {
			fmt.Fprintf(&b, " description %s\n", p.Desc)
		}
		switch {
		case p.Trunk:
			b.WriteString(" switchport mode trunk\n")
		case p.Mode == "dynamic auto": // the default: not shown
			fmt.Fprintf(&b, " switchport access vlan %d\n", p.Vlan)
		case p.Mode != "":
			fmt.Fprintf(&b, " switchport access vlan %d\n switchport mode %s\n", p.Vlan, p.Mode)
		default:
			fmt.Fprintf(&b, " switchport access vlan %d\n switchport mode access\n", p.Vlan)
		}
		b.WriteString("!\n")
	}
	return strings.ReplaceAll(d.render(d.static["show running-config"], now), "{{INTERFACES}}", b.String())
}

func (d *Device) switchport(p *Port) string {
	adm, op, nego, encap := "static access", "static access", "Off", "native"
	switch {
	case p.Trunk:
		adm, nego = "trunk", "On"
	case p.Mode != "":
		adm, nego = p.Mode, "On"
	}
	switch {
	case p.operTrunk():
		op, encap = "trunk", "dot1q"
	case !p.Up:
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
Operational Trunking Encapsulation: %s
Negotiation of Trunking: %s
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
Appliance trust: none`, p.Short, adm, op, encap, nego, p.Vlan, name)
}

// portState is what every interface format needs from a port.
type portState struct {
	st, proto, tag, desc string
	crc                  int
}

func stateOf(p *Port) portState {
	ps := portState{st: "up", proto: "up", tag: "connected", crc: len(p.Name) % 3}
	if !p.Up {
		ps.st, ps.proto, ps.tag = "down", "down", "notconnect"
	}
	if p.Desc != "" {
		ps.desc = "\n  Description: " + p.Desc
	}
	return ps
}

// iosIface renders "show interfaces X" in IOS / IOS-XE format.
func iosIface(p *Port) string {
	ps := stateOf(p)
	duplex := "Full-duplex, 1000Mb/s, media type is 10/100/1000BaseTX"
	if !p.Up {
		duplex = "Auto-duplex, Auto-speed, media type is 10/100/1000BaseTX"
	}
	if strings.HasPrefix(p.Name, "TenGig") {
		duplex = "Full-duplex, 10Gb/s, link type is auto, media type is SFP-10GBase-SR"
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
     0 output buffer failures, 0 output buffers swapped out`, p.Name, ps.st, ps.proto, ps.tag, p.MAC, p.MAC, ps.desc, duplex, ps.crc, ps.crc)
}

// logLine is a canned log message, ago before the device clock.
type logLine struct {
	ago time.Duration
	msg string
}
