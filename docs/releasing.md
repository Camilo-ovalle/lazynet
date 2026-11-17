# Releasing lazynet

This guide covers publishing a release, its release notes, and how users get
it: the install script and `lazynet update`.

## How the pieces fit

```
CHANGELOG.md ──► embedded in the binary ──► "What's new" banner, lazynet changelog

git tag v0.2.0 ──► release.yml
                   └─ goreleaser job ──► GitHub Release: tarballs, .deb, .rpm, checksums.txt,
                                         notes from the tag's CHANGELOG.md section

scripts/install.sh ──┐
lazynet update ──────┴──► latest GitHub Release ──► download, verify SHA-256, install
                          (the TUI asks at most once a day and offers the update)

pages.yml ──► landing page (site/, Astro + VHS recordings) ──► GitHub Pages
                                                              https://camilo-ovalle.github.io/lazynet
```

Users install with the script and update with `lazynet update` (or by
running the script again). Both do the same thing: download the release
tarball for their CPU, check it against `checksums.txt`, and replace the
binary where it is, with `sudo` only when that location needs root. The
`.deb` and `.rpm` packages remain available on each release; `lazynet update`
upgrades them with `dpkg` or `rpm`.

## One-time setup

### Enable GitHub Pages (landing page)

Go to *Settings → Pages → Build and deployment → Source* and select
**GitHub Actions**.

Then go to *Settings → Environments → `github-pages` → Deployment branches and
tags* and add a rule for the `main` branch. Without it, a deploy from `main`
fails with *Branch "main" is not allowed to deploy to github-pages due to
environment protection rules*.

### Clean up after the APT repository (once)

lazynet 0.1.0 also published a signed APT repository on GitHub Pages. It was
retired because it was hard to set up and confusing for most users. Once
pages.yml has deployed the site without it, you can delete what only the
repository used:

- the `gh-pages` branch (it only stored the repository files);
- the `APT_GPG_PRIVATE_KEY` and `APT_GPG_PASSPHRASE` repository secrets;
- the signing key itself, or keep its backup in your password manager.

Users who installed from it are told how to remove the old source in the
README and by `lazynet update`.

## Cutting a release

1. Write the release notes. In `CHANGELOG.md`, rename `## [Unreleased]` to
   `## [X.Y.Z] - YYYY-MM-DD`, review its entries (written for users, under
   `### Added`, `### Changed`, `### Removed`, `### Fixed` or `### Security`),
   and add a new empty `## [Unreleased]` section above it. Merge that change
   to `main` like any other (for example on a `chore/release-vX.Y.Z` branch).
   `go test ./internal/update` checks that the file still parses, and
   `sh scripts/release-notes.sh vX.Y.Z` previews the notes.
2. Make sure the CI is green on `main`. The `packaging` job has already built
   every package, installed lazynet with the install script (fresh install,
   rerun and update in place) and installed the `.deb`.
3. Tag the release and push the tag (semantic versioning: `vMAJOR.MINOR.PATCH`):

   ```sh
   git switch main && git pull
   git tag -a v0.2.0 -m "lazynet v0.2.0"
   git push origin v0.2.0
   ```

4. Watch the run in *Actions → release*. When it finishes, *Releases* shows
   the tarballs, `.deb`, `.rpm` and `checksums.txt`, and the notes from
   `CHANGELOG.md`. From then on, `lazynet update` and the install script
   install the new version, and the TUI offers it within a day.

If the release job fails before GoReleaser created the release, fix the
problem on `main`, then move the tag to the fixed commit and push it again:

```sh
git tag -d v0.2.0 && git push origin :refs/tags/v0.2.0
git tag -a v0.2.0 -m "lazynet v0.2.0" && git push origin v0.2.0
```

Never move a tag after its release was published: users may already have it.
Publish a new patch version instead.

## Release notes and updates

`CHANGELOG.md` is the single source of the release notes:

- **GitHub release notes.** `scripts/release-notes.sh vX.Y.Z` prints the
  section of a version. The release workflow passes it to GoReleaser and
  **fails before publishing anything** if the tag has no section, so a
  release can't go out without notes.
- **"What's new" in the TUI.** The file is embedded in the binary
  (`changelog.go`). When lazynet opens after an update, it shows the sections
  between the version that last ran and the running one (recorded in
  `~/.local/share/lazynet/update.json`). `lazynet changelog` prints them too.
  Because the notes travel inside the binary, this works with every install
  method and without internet access.
- **The update banner and `lazynet update`** read
  `https://api.github.com/repos/Camilo-ovalle/lazynet/releases/latest`
  (stable releases only: drafts and pre-releases are never offered). The
  TUI asks at most once a day, and never with `check_updates: false` or in
  demo mode. Only release builds (`X.Y.Z`) check; `make build` and
  `go run` builds are development builds and never do.

The install script and `lazynet update` download release files by name, so
keep these names stable in `.goreleaser.yaml`:
`lazynet_<version>_linux_<arch>.tar.gz` (with the `lazynet` binary at the
root), `lazynet_<version>_linux_<arch>.deb`,
`lazynet_<version>_linux_<arch>.rpm` and `checksums.txt`. Both refuse any
download whose SHA-256 is not listed in `checksums.txt`.

## Verifying a release

On a clean Linux machine, VM or container:

```sh
curl -fsSL https://raw.githubusercontent.com/Camilo-ovalle/lazynet/main/scripts/install.sh | sh
lazynet version
```

On a machine that still runs the previous release, check the update path:

```sh
lazynet update --check   # reports the new version
lazynet update           # installs it and prints what changed
lazynet                  # (on another machine) the banner offers the update; after it, "What's new"
```

Running the install script again on an up-to-date machine must say
`lazynet X.Y.Z is already installed`.
