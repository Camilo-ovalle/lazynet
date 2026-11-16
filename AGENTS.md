# AGENTS.md

Guidance for AI coding agents (Claude Code, Codex, Cursor, …) working on lazynet.

## Communication

- The maintainer is a Spanish-speaking network sysadmin (Cisco Catalyst, Nexus, ASA). **Reply in Spanish.**
- Write everything in the repository in English: code, identifiers, UI text, error messages, commit messages and this file. The only exception is `README.es.md`.
- `README.md` and `README.es.md` are full translations of each other. When you change one, update the other.

## What lazynet is

A Go terminal tool, inspired by lazygit and lazydocker, for operating Cisco devices over SSH:
- `lazynet` with no arguments opens a Bubble Tea TUI with mouse support;
- subcommands (`info`, `iface`, `logs`, `backup`, `vlan set`, `inventory`) are for scripting, with `-o json` available.

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
| `internal/app` | Connection flow, credential prompting, backups, VLAN orchestration, `--demo` |
| `internal/device` | Platform detection, drivers (catalyst/nexus/asa), interface-name normalization, VLAN change planning |
| `internal/transport` | scrapligo session over crypto/ssh, read-only guard, `ChangeRequest`, host keys (TOFU) |
| `internal/parse` | Regex parsers for `show` output and log timestamps |
| `internal/mockssh` | SSH server that emulates Cisco CLI (tests, `--demo`, hidden `mock` command); fixtures in `fixtures/<platform>/` |
| `internal/creds` | Password cache with TTL (Linux kernel keyring) |
| `internal/backup` | go-git repository of running-configs, diffs |
| `internal/audit` | JSONL audit log |
| `internal/inventory`, `internal/config` | YAML inventory and settings, XDG paths (`LAZYNET_HOME` override) |
| `internal/render`, `internal/theme` | Output rendering shared by the CLI and the TUI; Lip Gloss palette |
| `assets/` | README images and their generator (`assets/assets-generator/README.md`) |
| `site/` | Landing page (Astro, Node 24). Content and real command outputs are in `site/src/data/content.ts`; VHS recordings in `site/tapes/` |

## Safety rules (non-negotiable)

These rules are the point of the project. Do not weaken them, and do not add ways around them.

1. **Everything is read-only except the access-VLAN change.**
   - `transport.CheckReadOnly` only allows `show …` and session settings (`terminal length 0`, `terminal width N`, `terminal pager 0`).
   - It rejects control characters, `| redirect`, `| tee`, `| append`, `| save` and `>`.
   - A new feature must use `show` commands only.
2. **Configuration changes only go through `transport.ChangeRequest`.**
   - The request must be confirmed with `Confirm(shown)`, where `shown` is exactly what was displayed to the user.
   - `Apply` refuses an unconfirmed request.
   - There is no `--yes` flag, and none should be added.
3. **VLAN change flow (Catalyst only).** The steps run in this order:
   1. `device.NormalizeInterface` validates the port strictly; it is the injection barrier.
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

Out of scope for now: restoring backups, a syslog server as a log source, `enable` secret, jump hosts, VLAN changes on Nexus/ASA.

## Testing conventions

- Device behaviour is tested against `internal/mockssh`, which emulates prompts, modes, pipes and stateful ports and VLANs. To support new real-device output, add it as a fixture under `internal/mockssh/fixtures/<platform>/`, sanitized (no real serials, IPs or hostnames), and cover it in `internal/parse/parse_test.go`.
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

## Status and open items (as of 2026-11-15)

- **v0.1.0 is released** (2026-11-16). Every install method was verified against it:
  - the APT repository on GitHub Pages (`https://camilo-ovalle.github.io/lazynet`, suite `stable`, component `main`);
  - `install.sh`, `go install`, and the .deb, .rpm and tarball downloads.

  The repository signing key fingerprint is `7165 840B 33CF 585D 3786  84A7 935A 8803 6A92 6BB6`; the README publishes it.
  Releases: tag `vX.Y.Z` and push it. If the APT publication fails, rerun it with *Actions → release → Run workflow* and the tag. See `docs/releasing.md`.
- The landing page (`site/`) and the APT repository share GitHub Pages. `pages.yml` builds the site, records the VHS demos, merges the APT files from `gh-pages` (storage only) and checks that `InRelease` still verifies before deploying. Pages source must be *GitHub Actions*, and the `github-pages` environment must allow deployments from `main`.
- Licensed under Apache-2.0, copyright Camilo Enrique Ovalle Ceron (`LICENSE`, `NOTICE`). The repository was renamed from `Lazy_Net` to `lazynet`; the module path is `github.com/Camilo-ovalle/lazynet`.
- The repository is public.
- No tests on real devices yet. The Catalyst model and OS version are still unknown; the maintainer will share them.
- Lab plan: CML-Free on VMware Workstation (Windows host, WSL2 dev environment), with an IOL-L2 (IOS XE 17.18) switch. Its ports are named `Ethernet0/x`, so use `-p e0/1`.
  - VMware NAT network (VMnet8): `192.168.172.0/24`, gateway `.2`.
  - CML server: `.10`. Switch `lab-sw1`: `.50`.
  - The maintainer's Cisco account was on compliance hold, waiting to download CML.
- The TUI still needs a visual polish pass.
