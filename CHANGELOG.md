# Changelog

Every lazynet release is listed here, newest first. lazynet embeds this file:
after an update, the terminal UI shows the sections between the version you
had and the one you have now, and `lazynet changelog` prints them. The release
workflow also publishes the section of each tag as its GitHub release notes.

Write entries for users: what they can do now, or what works better. Group
them under `### Added`, `### Changed`, `### Removed`, `### Fixed` or
`### Security`, and keep
each entry to one or two lines. Before tagging `vX.Y.Z`, rename
`## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` and start a new empty
`## [Unreleased]` section above it (see `docs/releasing.md`).

## [Unreleased]

### Added

- `lazynet update` updates lazynet to the latest release, the same way it was
  installed: it replaces the binary installed by the install script, or
  upgrades the `.deb` or `.rpm` package. `--check` only reports whether a
  newer version exists.
- The terminal UI tells you when a new release is out: update now, skip that
  version, or decide later. After updating, it shows what changed.
- `lazynet changelog` prints the release notes of every version.
- `check_updates: false` in `config.yml` stops lazynet from contacting GitHub.
- Palo Alto firewalls (PAN-OS): device info and XML configuration backups.
- The inventory accepts `paloalto` (or `panos`) as a platform.

### Changed

- The install script is the recommended way to install lazynet. Running it
  again updates lazynet where it is, or does nothing if it is up to date.
- The terminal UI uses the brand palette, a full-width status bar and a
  two-column help screen.
- The terminal UI only offers the features the selected device supports.

### Removed

- The APT repository. If you installed from it, remove its source with
  `sudo rm /etc/apt/sources.list.d/lazynet.list /usr/share/keyrings/lazynet.gpg`,
  or `apt update` keeps reporting an error for it. `lazynet update` keeps
  working for your installed package.

### Fixed

- Typing in a prefilled field replaces its value instead of appending to it.
- The device information stays visible after a VLAN change, and the port is
  read again from the device.

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
