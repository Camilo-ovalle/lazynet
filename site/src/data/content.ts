// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Site content. Command outputs are real captures from `lazynet --demo`
// (emulated devices). Refresh them when the CLI output changes.

export const repo = "https://github.com/Camilo-ovalle/lazynet";

export interface CommandTab {
  id: string;
  label: string;
  title: string;
  description: string;
  command: string;
  output: string;
}

export const commands: CommandTab[] = [
  {
    id: "info",
    label: "info",
    title: "Device info at a glance",
    description:
      "Model, serial, MAC, IP, OS version, uptime and clock. Works for one device or a whole inventory group in parallel, with -o json for scripts.",
    command: "lazynet info core-sw1",
    output: `core-sw1  CAT IOS-XE 17.09.04a
  Hostname    core-sw1
  Model       C9300-48P
  Serial      FCW2318L0AB
  MAC         706d.15a2.3b80
  IP          10.10.0.11
  OS version  IOS-XE 17.09.04a
  Uptime      12 weeks, 3 days, 4 hours, 51 minutes
  Clock       2026-11-16 01:18:30 (device local)`,
  },
  {
    id: "iface",
    label: "iface",
    title: "One port, everything that matters",
    description:
      "Status, access VLAN, speed, error counters and description. Shorthand like 1/0/25 or gi1/0/25 expands to the full interface name.",
    command: "lazynet iface core-sw1 -p 1/0/25",
    output: `GigabitEthernet1/0/25
  Status       down (notconnect)
  Protocol     down
  Description  Camera-Parking
  Mode         access
  Access VLAN  10 DATA
  Speed        Auto-speed auto-duplex
  MAC          706d.15a2.3b99
  Errors       in 0 · crc 0 · out 0
  Input rate   12000 bits/sec (5 minute)
  Output rate  45000 bits/sec (5 minute)`,
  },
  {
    id: "logs",
    label: "logs",
    title: "Logs by time range, not by scrolling",
    description:
      "Filter the device log buffer with --from/--to or --last, plus text and severity filters. Understands IOS, IOS-XE, NX-OS and ASA timestamps.",
    command: "lazynet logs core-sw1 --last 24h --severity warning",
    output: `2026-11-15 16:18:30 WRN SW_MATM-4-MACFLAP_NOTIF Host 0050.56a1.2b3c in vlan 10 is flapping between port Gi1/0/3 and port Gi1/0/4
2026-11-15 20:18:30 WRN SEC_LOGIN-4-LOGIN_FAILED Login failed [user: root] [Source: 10.0.0.66] [localport: 22] [Reason: Login Authentication Failed]
2026-11-15 23:18:30 ERR LINK-3-UPDOWN Interface GigabitEthernet1/0/25, changed state to down
2026-11-15 23:48:30 ERR LINK-3-UPDOWN Interface GigabitEthernet1/0/25, changed state to up
4 of 12 entries · device clock 2026-11-16 01:18:30`,
  },
  {
    id: "backup",
    label: "backup",
    title: "Versioned config backups",
    description:
      "running-config snapshots in a local git repository. A new version only when something changed, and unified diffs between any two.",
    command: "lazynet backup --all",
    output: `✓ core-sw1             new version 0c4a7cd3
✓ access-sw2           new version d16feef3
✓ dc-n9k-1             new version 81367f4f
✓ fw-edge              new version 969ee8d5
store: ~/.local/share/lazynet/backups`,
  },
  {
    id: "vlan",
    label: "vlan set",
    title: "The one change it can make, made safely",
    description:
      "Validated read-only first, then the exact commands and an explicit y. Verified on the device, saved only if you say so, and always audited.",
    command: "lazynet vlan set core-sw1 -p 1/0/7 -v 20",
    output: `checking port and VLAN (read-only)...

  Device   core-sw1 (10.10.0.11)  CAT
  Port     GigabitEthernet1/0/7
  Current  VLAN 110 PRINTERS
  Target   VLAN 20 VOICE

The following commands will be sent to the device:
  configure terminal
  interface GigabitEthernet1/0/7
  switchport access vlan 20
  end

Apply this change? [y/N]: y
✓ GigabitEthernet1/0/7 is now in VLAN 20 (verified on the device).

Save running-config to startup-config? This will send:
  write memory
Save configuration? [y/N]: y
✓ Configuration saved.`,
  },
];

export interface InstallTab {
  id: string;
  label: string;
  note: string;
  code: string;
}

export const installs: InstallTab[] = [
  {
    id: "script",
    label: "Script",
    note: "Any Linux distribution. Downloads the latest release, verifies its SHA-256 and installs it. Run it again to update.",
    code: `curl -fsSL https://raw.githubusercontent.com/Camilo-ovalle/lazynet/main/scripts/install.sh | sh`,
  },
  {
    id: "rpm",
    label: "RPM",
    note: "Fedora, RHEL, Rocky and Alma. Replace the version with the latest release.",
    code: `sudo dnf install ${repo}/releases/download/v0.1.0/lazynet_0.1.0_linux_amd64.rpm`,
  },
  {
    id: "go",
    label: "Go",
    note: "Requires Go 1.26 or newer. Installs into $(go env GOPATH)/bin.",
    code: `go install github.com/Camilo-ovalle/lazynet/cmd/lazynet@latest`,
  },
];

export const personas = [
  {
    icon: "terminal",
    title: "Network administrators",
    text: "Check a port, a serial or last night's logs without opening five SSH sessions or remembering the syntax of each platform.",
  },
  {
    icon: "pulse",
    title: "NOC and on-call",
    text: "Read-only by design, so anyone on the rotation can use it. One keyboard-driven view across Catalyst, Nexus and ASA.",
  },
  {
    icon: "shield",
    title: "Change control and audit",
    text: "Every VLAN change shows its exact commands, needs an explicit yes, is verified on the device and lands in a JSON audit log.",
  },
];

export const showcase = [
  {
    eyebrow: "Terminal UI",
    title: "Lazygit-style navigation for your network",
    text: "Devices grouped from your inventory on the left, Info, Interface, Logs and Backups tabs on the right. Drive it with the keyboard, or click and scroll with the mouse.",
    points: ["j/k, tab and 1–4 to move", "Click devices and tabs, scroll with the wheel", "Async SSH: the UI never freezes"],
    media: "tui",
    poster: "brand/lazynet-mockup-terminal.png",
    alt: "lazynet TUI showing device info for a Catalyst 9300",
  },
  {
    eyebrow: "Safe changes",
    title: "See the exact commands before anything runs",
    text: "The VLAN change refuses trunks, missing VLANs and reserved ranges before showing you a single command. Enter never approves; only an explicit y does.",
    points: ["Read-only validation first", "Verified on the device after applying", "write memory is a separate question"],
    media: "vlan",
    poster: "brand/lazynet-mockup-vlan.png",
    alt: "VLAN change confirmation showing the exact commands to be sent",
  },
];

export const guarantees = [
  { title: "Read-only transport", text: "The SSH layer only sends show commands and session settings. Anything else, including | redirect and > file, is rejected before it reaches a device." },
  { title: "Exact commands, explicit yes", text: "Changes run only through a request confirmed with the very lines you were shown. There is no --yes flag." },
  { title: "Verified and audited", text: "The port is read back after every change, and each attempt (declined, applied or failed) is appended to a JSON audit log." },
  { title: "Passwords never on disk", text: "Prompted like sudo, remembered for 5 minutes in the Linux kernel keyring, which expires them on its own." },
  { title: "Host key before password", text: "Trust-on-first-use host keys, verified before a password is ever typed. A changed key is a hard error." },
  { title: "Privilege 15 or nothing", text: "Accounts that land at > get a clear error instead of a half-working session." },
];

export const features = [
  { title: "Automatic platform detection", text: "Point it at an IP; it works out Catalyst (IOS / IOS-XE), Nexus (NX-OS), ASA or a Palo Alto firewall." },
  { title: "Palo Alto backups", text: "Device info and versioned PAN-OS configuration backups in native XML, the format the firewall imports." },
  { title: "Demo mode", text: "lazynet --demo starts emulated devices on localhost. Try every feature without a network." },
  { title: "Single static binary", text: "SSH, git and the keyring are built in. No Python, no agents, no runtime dependencies." },
  { title: "Inventory with groups", text: "A YAML inventory with groups, plus ad-hoc IPs. Run info or backup across a whole group in parallel." },
  { title: "Scriptable", text: "Every command has -o json, and shell completion even suggests device names from your inventory." },
];
