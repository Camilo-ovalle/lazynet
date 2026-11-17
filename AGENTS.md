# AGENTS.md

Guidance for AI coding agents (Claude Code, Codex, Cursor, …) working on lazynet.

## Communication

- The maintainer is a Spanish-speaking network sysadmin (Cisco Catalyst, Nexus, ASA). **Reply in Spanish.**
- Write everything in the repository in English: code, identifiers, UI text, error messages, commit messages and this file. The only exception is `README.es.md`.
- `README.md` and `README.es.md` are full translations of each other. When you change one, update the other.

## What lazynet is

A Go terminal tool, inspired by lazygit and lazydocker, for operating network devices over SSH (Cisco Catalyst, Nexus and ASA; Palo Alto info and backups):
- `lazynet` with no arguments opens a Bubble Tea TUI with mouse support;
- subcommands (`info`, `iface`, `logs`, `backup`, `vlan set`, `inventory`) are for scripting, with `-o json` available;
- `update` installs the latest release and `changelog` prints the release notes.

It targets Linux first. Windows and PowerShell come later.

## Commands

```sh
make test          # go test ./...  (CI also runs -race)
make vet           # go vet + gofmt check
make build         # static binary ./lazynet (CGO_ENABLED=0)
make demo          # TUI against emulated devices
go run ./cmd/lazynet --demo info core-sw1     # any subcommand works with --demo
go run ./cmd/lazynet mock --platform catalyst --listen 127.0.0.1:2222
```

CI (`.github/workflows/ci.yml`) runs gofmt, `go vet`, `go test -race`, a static build and `shellcheck scripts/install.sh`. Run `gofmt -l .`, `go vet ./...` and `go test ./...` before you commit.

## Layout

| Path | Role |
|---|---|
| `cmd/lazynet` | Entry point (`cli.Execute`) |
| `internal/cli` | Cobra subcommands, terminal prompter (reads `/dev/tty`) |
| `internal/tui` | Bubble Tea model, view, modals, async ops, prompter bridge |
| `internal/app` | Connection flow, credential prompting, capability checks (`Session.Interface/Logs/RunningConfig`), backups, VLAN orchestration, `--demo` |
| `internal/platform` | The platform contract: `Definition`, registry, `Driver` and capability interfaces, shared data types (`Info`, `InterfaceDetail`, `LogResult`, `VlanPlan`), `Dialect()`. Names no vendor |
| `internal/platform/platformtest` | Test helpers for platform packages: `Dial` (emulator over SSH), `Shower` (in memory), `ConfigCommands` |
| `internal/platforms/all` | The only list of platforms (blank imports) and the cross-platform contract tests |
| `internal/platforms/cisco` | Helpers shared by the Cisco platforms: failure markers, version probe, `NormalizeInterface`, clock/log/interface readers |
| `internal/platforms/cisco/{catalyst,nexus,asa}`, `internal/platforms/paloalto` | One package per device family: `Definition`, driver, platform-specific parsers, tests |
| `internal/detect` | Detection pipeline (pinned platform, then the registered probes by priority) |
| `internal/transport` | Vendor-neutral scrapligo session over crypto/ssh, `Dialect`, read-only guard, `ChangeRequest`, host keys (TOFU) |
| `internal/parse` | Parsers and types shared by several platforms (interfaces, switchports, VLANs, logs, clocks, MACs) |
| `internal/mockssh` | SSH server that emulates each platform's CLI (tests, `--demo`, hidden `mock` command). One emulator file per platform (`catalyst.go`, `paloalto.go`, …; Cisco CLIs share `ios.go`); fixtures in `fixtures/<platform>/` |
| `internal/creds` | Password cache with TTL (Linux kernel keyring) |
| `internal/update` | Release check (GitHub API, cached daily in `update.json`), `lazynet update` per install method (binary from the install script, `.deb`, `.rpm`), release notes parsed from the embedded `CHANGELOG.md` |
| `CHANGELOG.md`, `changelog.go` | Release notes, embedded in the binary (root package `lazynet`) for the TUI "What's new" banner and `lazynet changelog`; also the GitHub release notes |
| `internal/backup` | go-git repository of running-configs, diffs |
| `internal/audit` | JSONL audit log |
| `internal/inventory`, `internal/config` | YAML inventory and settings, XDG paths (`LAZYNET_HOME` override) |
| `internal/render`, `internal/theme` | Output rendering shared by the CLI and the TUI; Lip Gloss palette |
| `assets/` | README images and their generator (`assets/assets-generator/README.md`) |
| `site/` | Landing page (Astro, Node 24). Content and real command outputs are in `site/src/data/content.ts`; VHS recordings in `site/tapes/` |

## Architecture: one module per platform

Every device family is a self-contained package under `internal/platforms/` that registers a `platform.Definition` from `init()`. The rest of lazynet only talks to the registry, so **adding or changing a platform never touches the CLI, the TUI, the transport, the theme or another platform**.

- **No vendor names outside the platform packages.** Don't add `switch` statements, `if id == "…"` checks or vendor strings to `app`, `cli`, `tui`, `render`, `theme`, `transport`, `detect` or `inventory`. Help texts, badges, the `--platform` lists, the demo devices and the transport's failure markers all come from the registry (`platform.All`, `Names`, `Describe`, `With`, `Dialect`).
- **Features are capabilities, not platform checks.** A driver implements `platform.Driver` (device info) and, optionally, `InterfaceReader`, `LogReader`, `ConfigReader` (backups) and `VlanChanger`. `Definition.Supports(c)` is derived from the driver's type, so it can't drift. The CLI and the TUI offer a feature only when the platform supports it (`platform.Supports`, `app.Session.Interface/Logs/RunningConfig`), and report `platform.Unsupported(id, c)` otherwise. To support logs on a new platform, implement `Logs()`; nothing else changes.
- **Detection is declarative.** Each platform declares `Probes`: a read-only command, an optional prompt filter and a match function, with a priority (lower first; put specific matches before broad ones, e.g. ASA 10, Nexus 20, Catalyst 30). `internal/detect` runs them in order, runs a shared command once, skips probes whose prompt filter fails, and continues past commands the device rejects (`transport.ErrRejected`). A platform pinned in the inventory skips detection.
- **The transport is vendor-neutral.** It receives a `transport.Dialect` built from the registry: failure markers, the exact session setup commands, and the operational `>` prompts. The base prompt patterns in `internal/transport/cli.yaml` are generic (`name>`, `name#`, `name(config…)#`). If a vendor's prompt doesn't fit them, extending the patterns is a transport change that needs a safety review.
- **Shared code goes one level up, never sideways.** Code shared by platforms of one vendor goes in the vendor package (`internal/platforms/cisco`); parsers shared across vendors go in `internal/parse`. A platform package never imports another platform package.

### Adding a platform (checklist)

1. Create `internal/platforms/<vendor>/<family>/` (or `internal/platforms/<family>/` for a single-family vendor), with:
   - `<family>.go`: `const ID platform.ID = "<id>"` (lower case, stable forever), the `init()` that calls `platform.Register`, and the driver. Fill in `Name`, `Vendor`, `OS`, `Aliases`, `Order`, `Badge` (`Light` must keep about 4.5:1 contrast on white), `Probes`, `Session` and `Demo`.
   - `parse.go`: the parsers only this platform uses.
   - Tests: parsers against fixtures, and the driver against the emulator with `platformtest.Dial` (assert `platformtest.ConfigCommands(dev)` is empty after read-only operations).
2. Add one blank import to `internal/platforms/all/all.go`.
3. Add an emulator `internal/mockssh/<id>.go` (`register("<id>", …)`) and sanitized fixtures in `internal/mockssh/fixtures/<id>/` (the file `show_version.txt` answers `show version`). It gives tests, `--demo` and `lazynet mock --platform <id>` for free.
4. If the platform needs a paging command, add it to `Session.Setup` **and** to `reviewedSessionCommands` in `internal/platforms/all/all_test.go`, after checking it only affects the current session (see the safety rules).
5. Update the extra expectations in `internal/platforms/all/all_test.go` (`TestCapabilities`), the READMEs' support matrix, and the trademark notice in `NOTICE` and `site/src/components/Footer.astro` for a new vendor.
6. Run `go test ./...`. The contract tests in `internal/platforms/all` check every platform: probes are read-only, an emulator and a demo device exist, each emulator is detected as its own platform with every probe competing, and only read-only commands reach it.

## Safety rules (non-negotiable)

These rules are the point of the project. Do not weaken them, and do not add ways around them.

1. **Everything is read-only except the access-VLAN change.**
   - `transport.CheckReadOnly` only allows `show …` and the exact session setup commands the platforms declare (`Session.Setup`, today `terminal length 0`, `terminal width 511`, `terminal pager 0` and `set cli pager off`).
   - It rejects control characters, `| redirect`, `| tee`, `| append`, `| save` and `>`.
   - `Dial` refuses a dialect whose session commands contain pipes, redirections, separators or control characters, or start with a configuration verb (`configure`, `write`, `copy`, `commit`, `reload`, …), and refuses a dialect without failure markers.
   - The full list of session commands is pinned in `reviewedSessionCommands` (`internal/platforms/all/all_test.go`). Changing it is a safety review: only per-session paging or width settings belong there.
   - Detection probes must be `show` commands (a contract test enforces it).
   - A new feature must use `show` commands only.
2. **Configuration changes only go through `transport.ChangeRequest`.**
   - The request must be confirmed with `Confirm(shown)`, where `shown` is exactly what was displayed to the user.
   - `Apply` refuses an unconfirmed request.
   - There is no `--yes` flag, and none should be added.
3. **VLAN change flow (Catalyst only, `platform.VlanChanger`).** The steps run in this order:
   1. The driver validates the port strictly before using it (`cisco.NormalizeInterface`); it is the injection barrier. Never interpolate user input into a command without such a validator.
   2. Read-only checks: refuse trunk and dynamic ports, refuse VLANs that don't exist or aren't active, refuse reserved ranges.
   3. Show the exact commands and require an explicit `y`. `Enter` must never approve a dangerous change.
   4. Apply the change.
   5. Verify the port on the device.
   6. Ask separately before `write memory`.
   7. Write an audit record for every outcome: declined, applied or failed.
   
   The TUI re-plans right before applying and aborts if the state changed.
4. **Credentials are never written to disk.**
   - Passwords are prompted and kept for a 5-minute sliding grace period in the kernel keyring (`creds.Manager`).
   - The maintainer explicitly rejected environment variables and keyring-only storage.
5. **Host keys are checked before passwords.**
   - `transport.CheckHostKey` probes the key without authenticating, so a password is never typed for an unknown host.
   - A changed key is a hard error.
6. **Privilege 15 is required.** A `>` prompt must produce a clear error (`ErrNotPrivileged`).
   - The only exception is a platform's operational prompt (`Session.OperationalPrompt`), today PAN-OS `user@host>` or `user@host(state)>` (`paloalto.IsPrompt`). Cisco hostnames cannot contain `@`, so a Cisco user-EXEC login is still refused.
   - A session opened at an operational prompt is read-only: `Conn.Apply` refuses every change on it (`ErrChangesUnsupported`).

Out of scope for now: restoring backups, a syslog server as a log source, `enable` secret, jump hosts, VLAN changes on Nexus/ASA, Palo Alto interfaces/logs and any change on Palo Alto.

## Release notes (`CHANGELOG.md`)

`CHANGELOG.md` is what users read: the TUI shows it after an update ("What's new"), `lazynet changelog` prints it, and the release workflow publishes each tag's section as the GitHub release notes.

- **Every user-visible change adds an entry under `## [Unreleased]` in the same branch.** Group entries under `### Added`, `### Changed`, `### Removed`, `### Fixed` or `### Security`. Write for a network admin, one or two lines each: what they can do now, not how the code changed. Refactors, tests, CI and docs-only changes need no entry.
- **When cutting a release**, rename `## [Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD` and add a new empty `## [Unreleased]` above it, before tagging (see `docs/releasing.md`). The release workflow fails if the tag has no section (`scripts/release-notes.sh`).
- Keep the format parseable: versions newest first, `X.Y.Z` only, dates as `YYYY-MM-DD`. `TestEmbeddedChangelog` (`internal/update`) enforces it.
- `lazynet update` downloads release files by name. Don't rename `lazynet_<version>_linux_<arch>.{tar.gz,deb,rpm}` or `checksums.txt` in `.goreleaser.yaml`. Downloads are refused unless their SHA-256 is in `checksums.txt`; never add a way to skip that check.
- The update check only runs in release builds (`X.Y.Z`), never in `--demo` and never with `check_updates: false`. A failed check stays silent. The banner only opens when no operation, dialog or VLAN change is in progress (`flushNotices`); keep it that way.
- To try the update flow end to end without GitHub, build two binaries with `-ldflags "-X …/internal/cli.Version=0.1.0 -X …/internal/update.API=http://127.0.0.1:PORT/repo"`, serve a fake `repo/releases/latest` JSON and the release files locally, and use `LAZYNET_HOME` so the real state is untouched. Never run `lazynet update` on the binary you work with.

## Testing conventions

- Device behaviour is tested against `internal/mockssh`, which emulates prompts, modes, pipes and stateful ports and VLANs. Platform packages use it through `internal/platform/platformtest`.
- Tests live next to the code they cover: a platform's parsers and driver are tested in its own package; cross-platform behaviour (detection with every probe competing, the reviewed session commands, capabilities) in `internal/platforms/all`; transport tests use a local `testDialect`, because transport cannot import the platforms.
- To support new real-device output, add it sanitized (no real serials, IPs or hostnames) either as an emulator fixture (`internal/mockssh/fixtures/<platform>/`) or as parser test data (`internal/platforms/<…>/testdata/`), and cover it in that platform's tests.
- **Never touch the real keyring in tests or in demo mode.** Use `creds.NewManager(0)`.
- TUI tests (`internal/tui/tui_test.go`) drive the model synchronously through a harness. Commands slower than 400 ms are treated as timers and dropped. Keep cursors static and avoid real timers so the suite stays fast (about 0.6 s).
- `LAZYNET_SNAPSHOT=1 go test ./internal/tui -run TestTUIBrowse -v` prints a rendered screen.

## Website (`site/`)

- Astro 7, plain CSS with tokens from the brand palette (`site/src/styles/global.css`), and no UI framework. Keep the little JavaScript in `Base.astro` (tabs, copy, video play/pause).
- The layout takes its structure from k6.io, but the code, copy and graphics are all original. Don't copy third-party site code, text or artwork. Cisco platforms appear as text badges, never as logos.
- Command outputs in `content.ts` are real captures from `lazynet --demo`. Only the demo IP 127.0.0.1 is shown as 10.10.0.11. Recapture them when the CLI output changes. Never show commands or keys that don't exist.
- Recordings come from `site/tapes/*.tape` and are made in CI by `pages.yml`. If they're missing, `Media.astro` falls back to the mockup PNGs in `assets/`.
- Before committing site changes, run `cd site && npm run build`, then check desktop (1440px) and mobile (390px) screenshots for horizontal overflow.

## Licensing and authorship

- lazynet is a personal project of Camilo Enrique Ovalle Ceron, built on his own equipment and time. Keep it that way: never add employer data, configurations, hostnames, IPs or serials. Sanitize every fixture.
- Every source file starts with this header (`#` comments in shell scripts):

  ```go
  // Copyright 2026 Camilo Enrique Ovalle Ceron
  // SPDX-License-Identifier: Apache-2.0
  ```

  Keep a blank line between the header and `//go:build` or the package doc comment, so the header doesn't become package documentation.
- `NOTICE` must ship with every distribution (archives and packages already include it).

## Gotchas

- Bubble Tea `View()` has a value receiver, so state mutated there is lost. Compute layout and geometry in `Update` (see `ensureVisible`, `tabRanges`).
- Bubble Tea/termenv query the terminal background color at startup. Terminals that don't answer add a delay of about 5 seconds. An env-var override was tried and didn't help; don't retry it.
- Background goroutines that need user input go through the TUI prompter (`program.Send` plus a reply channel). Prompts queue behind open modals.
- Legacy SSH algorithms (CBC, group14-sha1, ssh-rsa) are offered after the modern ones. `legacy_ssh: false` disables them.
- Platform detection is cached in `~/.cache/lazynet/detected.json`, never written back into the user's inventory file. `inventory add` edits YAML through `yaml.Node` to keep comments.

## Git workflow

- **One descriptive branch per change, created from `main`**, for example `feat/…`, `fix/…`, `docs/…` or `chore/…`. Never commit directly to `main`.
- Use [Conventional Commits](https://www.conventionalcommits.org) with a structured body. Split unrelated changes into separate commits.
- The maintainer pushes and merges. Their SSH key has a passphrase, so give them the `git push` command instead of running it, and don't merge into `main` unless asked.
- Commit or push only when asked.

## Status and open items (as of 2026-11-16)

- **v0.1.0 is released** (2026-11-16). Every install method was verified against it:
  - the APT repository on GitHub Pages (`https://camilo-ovalle.github.io/lazynet`, suite `stable`, component `main`);
  - `install.sh`, `go install`, and the .deb, .rpm and tarball downloads.

  The repository signing key fingerprint is `7165 840B 33CF 585D 3786  84A7 935A 8803 6A92 6BB6`; the README publishes it.
  Releases: tag `vX.Y.Z` and push it. If the APT publication fails, rerun it with *Actions → release → Run workflow* and the tag. See `docs/releasing.md`.
- The landing page (`site/`) and the APT repository share GitHub Pages. `pages.yml` builds the site, records the VHS demos, merges the APT files from `gh-pages` (storage only) and checks that `InRelease` still verifies before deploying. Pages source must be *GitHub Actions*, and the `github-pages` environment must allow deployments from `main`.
- Licensed under Apache-2.0, copyright Camilo Enrique Ovalle Ceron (`LICENSE`, `NOTICE`). The repository was renamed from `Lazy_Net` to `lazynet`; the module path is `github.com/Camilo-ovalle/lazynet`.
- The repository is public.
- Target Catalyst in production: **C9200L-48P-4X on IOS XE 17.12.04** (ARM64, CAT9K_LITE_IOSXE; ports Gi1/0/1-48 and uplinks Te1/1/1-4). A synthetic `show version` fixture covers it (`internal/platforms/cisco/catalyst/testdata/`); replace it with a sanitized real capture when available. No tests on real devices yet.
- Lab plan: CML-Free on VMware Workstation (Windows host, WSL2 dev environment), with an IOL-L2 (IOS XE 17.18) switch. Its ports are named `Ethernet0/x`, so use `-p e0/1`.
  - VMware NAT network (VMnet8): `192.168.172.0/24`, gateway `.2`.
  - CML server: `.10`. Switch `lab-sw1`: `.50`.
  - The maintainer's Cisco account was on compliance hold, waiting to download CML.
- The platform code was split into per-platform modules (`internal/platforms/…`) with a registry, capability interfaces and a separate detector. User-visible behaviour is unchanged, except that the inventory now accepts `paloalto`/`panos` and the TUI only offers features the platform supports.
- Palo Alto support has not been tested on real hardware yet.
- Updates (unreleased, next version): `lazynet update`, `lazynet changelog`, the TUI update banner (update / skip this version / later) and the "What's new" banner, fed by `CHANGELOG.md`. Tested end to end against a local fake release server (binary method and TUI relaunch); the `.deb` and `.rpm` methods are covered by unit tests only, and no release has gone through it yet. v0.1.0 has no update check, so its users must update once by hand (`apt upgrade`, install script…); from the next release on, `lazynet update` works.
