# Changelog

Every lazynet release is listed here, newest first. lazynet embeds this file:
after an update, the terminal UI shows the sections between the version you
had and the one you have now, and `lazynet changelog` prints them. The release
workflow also publishes the section of each tag as its GitHub release notes.

Write entries for users: what they can do now, or what works better. Group
them under `### Added`, `### Changed`, `### Removed`, `### Fixed` or
`### Security`, and keep each entry to one or two lines. Before tagging
`vX.Y.Z`, rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` and start a
new empty `## [Unreleased]` section above it (see `docs/releasing.md`).

## [Unreleased]

## [0.2.0] - 2026-11-16

### Added

- Palo Alto firewalls (PAN-OS, HA pairs too): device info and XML
  configuration backups. Platform name `paloalto` (or `panos`).
- `lazynet update` installs the latest release the same way lazynet was
  installed, after checking its checksum. `--check` only looks.
- The terminal UI announces new releases: `u` update, `s` skip that
  version, `esc` later. After updating, it shows what changed.
- `lazynet changelog` prints the release notes.
- `check_updates: false` in `config.yml` for networks without internet.

### Changed

- The install script is the recommended install method. Run it again to
  update.
- Only supported features are offered per device: no VLAN changes on Nexus
  and ASA.
- New colours, a full-width status bar and a two-column help screen.
- Tested against the Catalyst C9200L-48P-4X on IOS XE 17.12.

### Removed

- The APT repository. If you used it, run
  `sudo rm /etc/apt/sources.list.d/lazynet.list /usr/share/keyrings/lazynet.gpg`.

### Fixed

- Changing a port's VLAN after querying another port failed with "invalid
  interface number". Prefilled fields are now replaced when you type.
- After a VLAN change, the Info tab no longer resets to "enter connect".

## [0.1.0] - 2026-11-16

### Added

- First release: a terminal UI and a CLI for Cisco Catalyst, Nexus and ASA
  over SSH.
- Device info, interface details, log buffer with time range, text and
  severity filters, and running-config backups in a local git repository.
- Access-VLAN changes on Catalyst, with read-only checks, an exact preview of
  the commands, explicit confirmation, verification and an audit log.
- Passwords kept for 5 minutes in the kernel keyring, never on disk.
- `--demo` mode with emulated devices.
- Install with APT, the install script, `.deb`, `.rpm`, a tarball or
  `go install`.
