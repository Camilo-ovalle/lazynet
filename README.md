# lazynet

A lazy terminal UI and CLI for Cisco **Catalyst** (IOS / IOS-XE), **Nexus** (NX-OS) and **ASA** —
inspired by lazygit and lazydocker.

*[Leer en español](README.es.md)*

- **Two ways to work:** run `lazynet` for an interactive TUI (keyboard + mouse), or use subcommands for quick, scriptable tasks.
- **Read-only by default:** everything uses `show` commands, enforced in code. The only change lazynet can make is an access-VLAN change, and it always shows the exact commands and asks for confirmation first.
- **Automatic platform detection:** connect to any device; lazynet figures out whether it is a Catalyst, Nexus or ASA.
- **Zero dependencies:** a single static binary. SSH, git (for backups) and the keyring are built in.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/Camilo-ovalle/Lazy_Net/main/scripts/install.sh | sh
```

Or with **apt** (Debian / Ubuntu):

```sh
curl -fsSL https://camilo-ovalle.github.io/Lazy_Net/lazynet.gpg | sudo tee /usr/share/keyrings/lazynet.gpg >/dev/null
echo "deb [signed-by=/usr/share/keyrings/lazynet.gpg] https://camilo-ovalle.github.io/Lazy_Net stable main" | sudo tee /etc/apt/sources.list.d/lazynet.list
sudo apt update && sudo apt install lazynet
```

Other options: download a `.deb`, `.rpm` or tarball from the [releases page](https://github.com/Camilo-ovalle/Lazy_Net/releases)
(`sudo apt install ./lazynet_*.deb`), or `go install github.com/Camilo-ovalle/Lazy_Net/cmd/lazynet@latest`.

## Try it without a network

```sh
lazynet --demo                      # TUI with emulated Catalyst, Nexus and ASA devices
lazynet --demo info --group core    # any subcommand works with --demo
```

Demo mode runs fake devices on localhost. Nothing touches your network, keyring or files.

## Quick start

```sh
lazynet inventory init        # creates ~/.config/lazynet/inventory.yml with examples
$EDITOR ~/.config/lazynet/inventory.yml
lazynet                       # open the TUI
```

### CLI

```sh
lazynet info core-sw1                                    # serial, MAC, IP, OS version, uptime
lazynet info --group core -o json
lazynet iface core-sw1 --port Gi1/0/25                   # status, VLAN, speed, errors (shorthand: -p 1/0/25)
lazynet logs core-sw1 --from "2026-11-01 08:00" --to "2026-11-01 18:00"
lazynet logs core-sw1 --last 2h --severity warning --grep UPDOWN
lazynet backup core-sw1        # or --group core, or --all
lazynet backup list core-sw1
lazynet backup diff core-sw1   # what changed since the previous backup
lazynet vlan set core-sw1 --port Gi1/0/1 --vlan 110
lazynet inventory list | add | check --reach | path
```

Devices can be inventory names, IPs or `host:port`. `-u user` overrides the username and `-o json` prints machine-readable output.
Shell completion (including device names) is installed with the packages, or run `lazynet completion bash|zsh|fish`.

### TUI keys

| Key | Action | Key | Action |
|---|---|---|---|
| `↑↓` / `j k` | move / scroll | `tab`, `h l` | switch pane |
| `1-4`, `[ ]` | Info · Interface · Logs · Backups | `enter` | connect / load |
| `p` | query an interface | `v` | change an access VLAN |
| `b` | take a backup | `a` | connect to an IP not in the inventory |
| `f` / `/` / `s` | logs: time range / text / severity | `r` | refresh |
| `?` | help | `q` | quit |

Mouse: click devices and tabs, scroll with the wheel.

## How the VLAN change stays safe

1. The port name is normalized and strictly validated (no command injection).
2. Read-only checks: `show interfaces <port> switchport` refuses trunks and dynamic ports, and `show vlan brief` checks that the VLAN exists and is active.
3. lazynet prints the **exact** commands (`configure terminal`, `interface …`, `switchport access vlan …`, `end`) and waits for an explicit `y`.
4. After applying, it reads the port back to verify the change.
5. It asks separately whether to run `write memory`.
6. Every attempt (declined, applied or failed) is appended to the audit log `~/.local/share/lazynet/audit.log` (JSON lines).

The transport layer rejects any command that isn't `show …` (or a session `terminal` setting) unless it comes from a change request confirmed with the same lines that were displayed. Output redirection such as `| redirect` and `> file` is blocked too.

## Credentials and SSH security

- Passwords are prompted, **never written to disk**, and remembered for a **5-minute grace period** (like `sudo`, refreshed on each use). On Linux they live in the kernel keyring with a kernel-enforced expiry, so the grace period also works across separate `lazynet` runs.
- Host keys follow trust-on-first-use and are stored in `~/.config/lazynet/known_hosts`. A changed key is a hard error. lazynet verifies the host *before* asking for a password.
- lazynet needs a privilege-15 login (`#` prompt). Accounts that land at `>` get a clear error.
- Old IOS images that only support CBC ciphers, `diffie-hellman-group14-sha1` or `ssh-rsa` work, because these legacy algorithms are offered after the modern ones. Disable them with `legacy_ssh: false`.

## Files

| Path | Content |
|---|---|
| `~/.config/lazynet/inventory.yml` | devices (`lazynet inventory init` writes an example) |
| `~/.config/lazynet/config.yml` | settings (optional) |
| `~/.config/lazynet/known_hosts` | trusted SSH host keys |
| `~/.local/share/lazynet/backups/` | git repository with one folder per device |
| `~/.local/share/lazynet/audit.log` | change audit log |

Inventory:

```yaml
defaults:
  username: admin
devices:
  - name: core-sw1
    host: 192.168.100.100
    group: core
    description: Core switch, rack A1
  - name: fw-edge
    host: 192.168.100.1
    group: firewalls
    username: fwadmin
    platform: asa        # optional: skip auto-detection
```

`config.yml` (all optional):

```yaml
credential_ttl: 5m     # 0 = never keep passwords outside the running process
workers: 5             # parallel sessions for --group / --all
ssh_timeout: 15s
command_timeout: 90s
default_username: ""
legacy_ssh: true
```

## Development

```sh
make test     # unit + integration tests against an emulated Cisco SSH server
make build
make demo
go run ./cmd/lazynet mock --platform catalyst --listen 127.0.0.1:2222   # standalone fake device
```

Releases are built by GoReleaser when a `v*` tag is pushed. `.github/workflows/release.yml` also publishes the signed APT repository to GitHub Pages
(it needs the `APT_GPG_PRIVATE_KEY` / `APT_GPG_PASSPHRASE` secrets).

## Roadmap

Windows / PowerShell support (Credential Manager), restoring backups, syslog-server log sources, `enable` secret support, jump hosts, and VLAN changes on Nexus.
