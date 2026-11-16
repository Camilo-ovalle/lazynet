<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/lazynet-lockup-dark-transparent.svg">
    <img src="assets/lazynet-lockup-light-transparent.svg" alt="lazynet" width="420">
  </picture>
</p>

<p align="center">
  <b>A terminal UI and CLI for Cisco Catalyst, Nexus and ASA, with Palo Alto firewall backups.</b><br>
  Inspect devices, read logs, back up configs and change access VLANs safely, from one terminal.
</p>

<p align="center">
  <a href="https://github.com/Camilo-ovalle/lazynet/actions/workflows/ci.yml"><img src="https://github.com/Camilo-ovalle/lazynet/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/Camilo-ovalle/lazynet/releases"><img src="https://img.shields.io/github/v/release/Camilo-ovalle/lazynet?include_prereleases&sort=semver" alt="Release"></a>
  <img src="https://img.shields.io/github/go-mod/go-version/Camilo-ovalle/lazynet" alt="Go version">
  <img src="https://img.shields.io/badge/platform-linux-informational" alt="Platform: Linux">
  <a href="LICENSE"><img src="https://img.shields.io/github/license/Camilo-ovalle/lazynet" alt="License"></a>
</p>

<p align="center">
  <a href="#installation">Installation</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="#usage">Usage</a> ·
  <a href="#safety-model">Safety model</a> ·
  <a href="#configuration">Configuration</a> ·
  <a href="README.es.md">Español</a>
</p>

<p align="center">
  <img src="assets/lazynet-mockup-terminal.png" alt="lazynet TUI showing device info for a Catalyst 9300" width="860">
</p>

## Why lazynet?

Answering "what VLAN is that port in?" or "what happened on this switch last night?" usually means opening an SSH session, finding the right `show` command for that platform, scrolling through output and copying values into a ticket. Changing an access VLAN adds the risk of a typo in configuration mode.

lazynet puts those everyday tasks behind a fast keyboard- and mouse-driven interface, in the spirit of [lazygit](https://github.com/jesseduffield/lazygit) and [lazydocker](https://github.com/jesseduffield/lazydocker). It is **read-only by design**. The only change it can make is an access-VLAN change. That change is validated first, then you see the exact commands and must confirm them. After it runs, lazynet verifies the result and writes an audit log entry.

## Table of contents

- [Features](#features)
- [Supported platforms](#supported-platforms)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Usage](#usage)
  - [Command line](#command-line)
  - [Terminal UI](#terminal-ui)
- [Safety model](#safety-model)
- [Configuration](#configuration)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [Roadmap](#roadmap)
- [Acknowledgments](#acknowledgments)
- [License](#license)

## Features

- **Two interfaces, one tool.** Run `lazynet` for an interactive TUI with keyboard and mouse support. Use subcommands for quick, scriptable tasks, with `-o json` for machine-readable output.
- **Device info at a glance.** Shows model, serial, MAC, management IP, OS version, uptime and clock, for one device or a whole inventory group in parallel.
- **Interface lookup.** Shows status, access VLAN, speed and duplex, error counters and description. Shorthand such as `1/0/25` or `gi1/0/25` is accepted.
- **Logs by time range.** Filters the device log buffer with `--from` / `--to` or `--last 2h`, plus text and severity filters. It understands IOS, IOS-XE, NX-OS and ASA timestamps, and infers missing years from the device clock.
- **Versioned config backups.** Stores `running-config` snapshots in a local git repository. A new version is created only when something changed, and lazynet shows unified diffs between versions.
- **Safe access-VLAN changes** on Catalyst switches. Trunk ports and missing VLANs are refused. You confirm the exact commands, the result is verified on the device, `write memory` is a separate prompt, and every attempt is audited.
- **Automatic platform detection.** Point it at an IP; lazynet works out whether it is a Catalyst, Nexus, ASA or Palo Alto firewall.
- **Palo Alto firewalls too.** Device info and versioned configuration backups (native PAN-OS XML, the format the firewall imports) over the same read-only SSH session.
- **Credentials handled like `sudo`.** Passwords are prompted and never written to disk. They are remembered for 5 minutes in the Linux kernel keyring.
- **Zero dependencies.** lazynet is a single static binary with SSH and git built in.
- **Demo mode.** `lazynet --demo` starts emulated devices on localhost, so you can try every feature without a network.

## Supported platforms

| Feature | Catalyst (IOS / IOS-XE) | Nexus (NX-OS) | ASA | Palo Alto (PAN-OS) |
|---|:---:|:---:|:---:|:---:|
| Platform detection | ✅ | ✅ | ✅ | ✅ |
| Device info | ✅ | ✅ | ✅ | ✅ |
| Interface lookup | ✅ | ✅ | ✅ | — |
| Logs by time range | ✅ | ✅ | ✅ | — |
| Config backup and diff | ✅ | ✅ | ✅ | ✅ (XML) |
| Access-VLAN change | ✅ | — | — | — |

Tested against Catalyst C9200L-48P-4X on IOS XE 17.12.04 output. On Palo Alto, lazynet logs in to operational mode and only runs `show` commands plus `set cli pager off`, a paging setting for its own CLI session.

lazynet runs on **Linux** (amd64 and arm64). Windows and PowerShell support is on the [roadmap](#roadmap).

> [!NOTE]
> lazynet is in early development. Parsers are tested against captured output from each platform. Testing on lab hardware is in progress, so please [open an issue](https://github.com/Camilo-ovalle/lazynet/issues) if output from your devices is not parsed correctly.

## Installation

lazynet is a single static binary with no runtime dependencies. Releases are published for **Linux amd64 and arm64**.

| Method | Best for | How you update |
|---|---|---|
| [APT repository](#apt-debian-ubuntu) (recommended) | Debian, Ubuntu and derivatives | `sudo apt upgrade` |
| [Install script](#install-script) | Any Linux distribution | Run the script again |
| [.deb / .rpm package](#deb-and-rpm-packages) | One-off installs, Fedora, RHEL | Install the newer package |
| [Tarball](#tarball) | Manual or offline installs | Replace the binary |
| [go install](#go-install) | Go developers | Run it again with `@latest` |
| [From source](#build-from-source) | Contributors | `git pull && make build` |

### APT (Debian, Ubuntu)

```sh
curl -fsSL https://camilo-ovalle.github.io/lazynet/lazynet.gpg | sudo tee /usr/share/keyrings/lazynet.gpg >/dev/null
echo "deb [signed-by=/usr/share/keyrings/lazynet.gpg] https://camilo-ovalle.github.io/lazynet stable main" | sudo tee /etc/apt/sources.list.d/lazynet.list
sudo apt update && sudo apt install lazynet
```

These commands do three things:
1. Save the repository's signing key.
2. Add the repository. `signed-by` makes that key valid only for this repository.
3. Install lazynet.

From then on, apt verifies every update against that key. The packages also install shell completions for bash, zsh and fish.

You can check that the key you downloaded is the right one:

```sh
gpg --show-keys /usr/share/keyrings/lazynet.gpg
# fingerprint: 7165 840B 33CF 585D 3786  84A7 935A 8803 6A92 6BB6
```

Upgrade, or uninstall:

```sh
sudo apt update && sudo apt upgrade lazynet
sudo apt remove lazynet
sudo rm /etc/apt/sources.list.d/lazynet.list /usr/share/keyrings/lazynet.gpg   # also remove the repository
```

### Install script

Works on any Linux distribution:

```sh
curl -fsSL https://raw.githubusercontent.com/Camilo-ovalle/lazynet/main/scripts/install.sh | sh
```

The [script](scripts/install.sh):
- detects your CPU architecture and downloads the latest release;
- verifies its SHA-256 checksum;
- installs the binary into `/usr/local/bin` (using `sudo` if needed) or into `~/.local/bin`.

| Variable | Effect |
|---|---|
| `LAZYNET_VERSION=0.1.0` | Install a specific version instead of the latest |
| `LAZYNET_INSTALL_DIR=$HOME/bin` | Install into another directory |

```sh
curl -fsSL https://raw.githubusercontent.com/Camilo-ovalle/lazynet/main/scripts/install.sh | LAZYNET_VERSION=0.1.0 sh
```

To upgrade, run the script again. To uninstall, delete the binary: `sudo rm "$(command -v lazynet)"`.

### .deb and .rpm packages

Use these to install a specific version without adding a repository. Find the version number on the [releases page](https://github.com/Camilo-ovalle/lazynet/releases).

```sh
VERSION=0.1.0
ARCH=amd64        # or arm64
BASE=https://github.com/Camilo-ovalle/lazynet/releases/download/v$VERSION
```

```sh
# Debian, Ubuntu
curl -fLO "$BASE/lazynet_${VERSION}_linux_${ARCH}.deb"
sudo apt install "./lazynet_${VERSION}_linux_${ARCH}.deb"

# Fedora, RHEL, Rocky, Alma
sudo dnf install "$BASE/lazynet_${VERSION}_linux_${ARCH}.rpm"
```

The packages install the binary into `/usr/bin`, along with shell completions and the license files. They don't add a repository, so updates are manual.

### Tarball

```sh
curl -fLO "$BASE/lazynet_${VERSION}_linux_${ARCH}.tar.gz"
curl -fLO "$BASE/checksums.txt"
sha256sum --check --ignore-missing checksums.txt
tar -xzf "lazynet_${VERSION}_linux_${ARCH}.tar.gz"
sudo install -m 0755 lazynet /usr/local/bin/lazynet
```

The archive also contains `LICENSE`, `NOTICE`, the READMEs and the shell completions (`completions/`).

### go install

Requires Go 1.26 or newer. This installs into `$(go env GOPATH)/bin`:

```sh
go install github.com/Camilo-ovalle/lazynet/cmd/lazynet@latest    # or @v0.1.0
```

### Build from source

Requires Go 1.26 or newer.

```sh
git clone https://github.com/Camilo-ovalle/lazynet.git
cd lazynet
make build        # produces ./lazynet
```

### Shell completion

The `.deb` and `.rpm` packages set this up for you. With any other method, generate it once. Completion also suggests device names from your inventory.

```sh
lazynet completion bash | sudo tee /usr/share/bash-completion/completions/lazynet >/dev/null
lazynet completion zsh  > "${fpath[1]}/_lazynet"
lazynet completion fish > ~/.config/fish/completions/lazynet.fish
```

### Check the installation

```sh
lazynet version
lazynet --demo     # try it right away with emulated devices
```

## Quick start

**1. Try it without a network.** Demo mode runs emulated Catalyst, Nexus and ASA devices on localhost. It doesn't touch your network, keyring or files.

```sh
lazynet --demo
```

**2. Create your inventory.**

```sh
lazynet inventory init          # writes an example to ~/.config/lazynet/inventory.yml
$EDITOR "$(lazynet inventory path)"
lazynet inventory check --reach # validates the file and tests TCP reachability
```

**3. Open the TUI.**

```sh
lazynet
```

## Usage

### Command line

```sh
lazynet info core-sw1                         # model, serial, MAC, IP, OS version, uptime
lazynet info --group core -o json             # a whole group, as JSON
lazynet iface core-sw1 -p 1/0/25              # one interface (Gi1/0/25, gi1/0/25 and 1/0/25 all work)
lazynet logs core-sw1 --last 2h --severity warning
lazynet logs core-sw1 --from "2026-11-01 08:00" --to "2026-11-01 18:00" --grep UPDOWN
lazynet backup --group core                   # or a single device, or --all
lazynet backup diff core-sw1                  # what changed since the previous backup
lazynet vlan set core-sw1 -p 1/0/7 -v 20      # validated, confirmed, verified, audited
```

| Command | Description |
|---|---|
| `info [device] [--group G]` | Device information |
| `iface <device> -p PORT [--raw]` | Interface details, or the raw `show` output |
| `logs <device> [--from T --to T \| --last D] [--grep S] [--severity L]` | Filtered device logs |
| `backup [device] [--group G \| --all]` | Save `running-config` into the backup repository |
| `backup list \| show \| diff <device>` | Browse and compare backups |
| `vlan set <device> -p PORT -v VLAN` | Change an access VLAN (Catalyst) |
| `inventory list \| add \| check \| init \| path` | Manage the inventory |
| `completion bash \| zsh \| fish` | Shell completion, including device names |

These options work with every command:
- `-u` to override the SSH username;
- `--ssh-port` to override the SSH port;
- `-o json` for machine-readable output;
- `--demo` to use the emulated devices.

A device can be an inventory name, an IP or `host:port`.

### Terminal UI

| Key | Action | Key | Action |
|---|---|---|---|
| `↑` `↓` / `j` `k` | Move, scroll | `tab` / `h` `l` | Switch pane |
| `1`–`4` / `[` `]` | Info · Interface · Logs · Backups | `enter` | Connect, load |
| `p` | Query an interface | `v` | Change an access VLAN |
| `b` | Take a backup | `a` | Connect to an IP not in the inventory |
| `f` · `/` · `s` | Logs: time range · text · severity | `r` | Refresh |
| `?` | Help | `q` | Quit |

You can also use the mouse: click devices and tabs, and scroll with the wheel.

## Safety model

<p align="center">
  <img src="assets/lazynet-mockup-vlan.png" alt="VLAN change confirmation showing the exact commands to be sent" width="720">
</p>

lazynet is meant to be safe to hand to anyone on a team. Its guarantees are enforced in code, not by convention.

**Read-only transport.** The SSH layer only sends `show …` commands and session settings (`terminal length 0`, `terminal width`, `terminal pager 0`). It rejects anything else before it reaches the device. That includes output redirection such as `| redirect`, `| tee` and `> file`, and control characters.

**The one change lazynet can make** is an access-VLAN change on a Catalyst switch:

1. The port name is normalized and strictly validated, so input like `1/0/1;reload` is rejected.
2. Read-only checks run first. `show interfaces <port> switchport` refuses trunk and dynamic ports. `show vlan brief` confirms the VLAN exists and is active, and reserved VLANs are refused.
3. lazynet shows the device, the port, the current and target VLAN, and the **exact** commands. It then waits for an explicit `y`. `Enter` alone never approves a change.
4. Only the lines you confirmed can be sent. The TUI re-checks the port right before applying; if anything changed since you reviewed it, nothing is sent.
5. lazynet reads the port back to verify the change.
6. A separate prompt asks before `write memory`. Until you save, the change exists only in `running-config`.
7. Every attempt (declined, applied or failed) is appended to `~/.local/share/lazynet/audit.log` as JSON lines. Each record includes the local user, device user, host, port, old and new VLAN, commands and result.

There is no `--yes` flag.

**Credentials and SSH.**
- Passwords are never written to disk. They are kept for a 5-minute sliding grace period in the Linux kernel keyring, which expires them on its own. Set `credential_ttl: 0` to keep them only in memory.
- Host keys follow trust-on-first-use and are stored in lazynet's own `known_hosts`. lazynet verifies the host key *before* it asks for a password. A changed key is a hard error.
- A privilege-15 login (`#` prompt) is required. Accounts that land at `>` get a clear error instead of a partial session.

## Configuration

lazynet follows the XDG base directory spec. Set `LAZYNET_HOME` to keep everything under one folder.

| Path | Content |
|---|---|
| `~/.config/lazynet/inventory.yml` | Devices and groups |
| `~/.config/lazynet/config.yml` | Settings (optional) |
| `~/.config/lazynet/known_hosts` | Trusted SSH host keys |
| `~/.local/share/lazynet/backups/` | Git repository with one folder per device |
| `~/.local/share/lazynet/audit.log` | Change audit log (JSON lines) |
| `~/.cache/lazynet/detected.json` | Cached platform detection |

<details>
<summary><b>inventory.yml</b></summary>

```yaml
defaults:
  username: admin
  # port: 22

devices:
  - name: core-sw1
    host: 10.10.0.11
    group: core
    description: Catalyst 9300 core switch, rack A1
  - name: dc-n9k-1
    host: 10.10.0.21
    group: datacenter
  - name: fw-edge
    host: 10.10.0.1
    group: firewalls
    username: fwadmin
    platform: asa        # optional: skips auto-detection (catalyst, nexus, asa, paloalto)
  - name: pa-fw1
    host: 10.10.0.2
    group: firewalls
```

`lazynet inventory add` appends a device and keeps your comments and formatting.

</details>

<details>
<summary><b>config.yml</b></summary>

```yaml
credential_ttl: 5m     # how long passwords are remembered; 0 = only while lazynet runs
workers: 5             # parallel sessions for --group / --all
ssh_timeout: 15s
command_timeout: 90s
default_username: ""
legacy_ssh: true       # also offer older algorithms (CBC, group14-sha1, ssh-rsa) after modern ones
```

</details>

## Troubleshooting

<details>
<summary><b>"logged in at user EXEC (>)"</b></summary>

lazynet needs privilege 15. Give the account `privilege 15` (or use AAA authorization that places it at level 15 on login). Entering `enable` mode is on the roadmap.

</details>

<details>
<summary><b>SSH negotiation fails on an old switch</b></summary>

Older IOS images may only offer CBC ciphers, `diffie-hellman-group14-sha1` or `ssh-rsa` host keys. These are enabled by default, after the modern algorithms. Check that `legacy_ssh` is not set to `false`, and that the device has an RSA key of at least 2048 bits (`crypto key generate rsa modulus 2048`).

</details>

<details>
<summary><b>"host key changed"</b></summary>

The device presented a different key than the one you trusted before. This can be legitimate, for example after an RMA or a key regeneration. Confirm with the device owner, then remove the old line from `~/.config/lazynet/known_hosts`.

</details>

<details>
<summary><b>Log entries "without date" are skipped</b></summary>

The device stamps logs with uptime instead of the date. Configure `service timestamps log datetime msec localtime year` so log lines can be filtered by date.

</details>

<details>
<summary><b>The TUI takes a few seconds to start</b></summary>

Some terminals don't answer the background-color query that the TUI sends at startup, so it waits for a short timeout. Modern terminals such as Windows Terminal, kitty, Alacritty and GNOME Terminal answer immediately.

</details>

## Development

```sh
make test     # unit and integration tests against emulated SSH devices
make build    # static binary in ./lazynet
make demo     # build and run the TUI in demo mode
go run ./cmd/lazynet mock --platform catalyst --listen 127.0.0.1:2222   # standalone fake device
```

<details>
<summary><b>Project layout</b></summary>

| Path | Role |
|---|---|
| `cmd/lazynet` | Entry point |
| `internal/cli` | Cobra subcommands |
| `internal/tui` | Bubble Tea interface |
| `internal/app` | Connection flow, credentials, backups, VLAN change orchestration |
| `internal/platform` | Platform contract and registry: drivers, optional capabilities, badges |
| `internal/platforms/…` | One package per device family (`cisco/catalyst`, `cisco/nexus`, `cisco/asa`, `paloalto`): driver, parsers and tests |
| `internal/detect` | Identifies the platform of a new session from the probes each platform declares |
| `internal/transport` | Vendor-neutral SSH sessions, read-only guard, confirmed change requests, host keys |
| `internal/parse` | Parsers shared by several platforms (interfaces, logs, clocks) |
| `internal/mockssh` | SSH server that emulates each platform's CLI (tests and demo mode) |
| `internal/backup`, `internal/audit`, `internal/creds` | Git backups, audit log, credential cache |
| `assets/` | Logo, icons and README images ([generator](assets/assets-generator)) |
| `site/` | Landing page ([camilo-ovalle.github.io/lazynet](https://camilo-ovalle.github.io/lazynet)), built with Astro |

</details>

Releases are built with [GoReleaser](https://goreleaser.com) when a `v*` tag is pushed. The release workflow also publishes the signed APT repository to GitHub Pages. See [docs/releasing.md](docs/releasing.md) for the one-time setup, the release steps and how the APT repository works.

Bug reports with sanitized `show` output from your devices are the most valuable contribution right now. They become parser test fixtures.

## Roadmap

- [ ] Windows and PowerShell support (Credential Manager)
- [ ] `enable` mode for accounts below privilege 15
- [ ] Jump hosts / bastions
- [ ] Logs from a syslog server
- [ ] Restoring backups
- [ ] VLAN changes on Nexus
- [ ] Palo Alto: interfaces and logs

## Acknowledgments

- [lazygit](https://github.com/jesseduffield/lazygit) and [lazydocker](https://github.com/jesseduffield/lazydocker), the inspiration for the interface.
- [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss) and [Bubbles](https://github.com/charmbracelet/bubbles) by Charm.
- [scrapligo](https://github.com/scrapli/scrapligo), [go-git](https://github.com/go-git/go-git) and [Cobra](https://github.com/spf13/cobra).

lazynet is an independent project. It is not affiliated with or endorsed by Cisco Systems or Palo Alto Networks. Cisco, Catalyst and Nexus are trademarks of Cisco Systems, Inc. Palo Alto Networks and PAN-OS are trademarks of Palo Alto Networks, Inc.

## License

Copyright 2026 Camilo Enrique Ovalle Ceron.

lazynet is licensed under the [Apache License, Version 2.0](LICENSE). It is provided **"as is", without warranties or conditions of any kind**. See the license for the full terms, including the limitation of liability. Redistributions must keep the copyright notice and the [NOTICE](NOTICE) file.
