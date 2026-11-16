# Releasing lazynet

This guide covers publishing a release and the APT repository that makes
`sudo apt install lazynet` work.

## How the pieces fit

```
git tag v0.1.0 ──► release.yml
                   ├─ goreleaser job ──► GitHub Release: tarballs, .deb, .rpm, checksums.txt
                   │                     (used by install.sh and manual downloads)
                   ├─ apt-repo job ────► downloads the release .debs, signs the repo
                   │                     ──► gh-pages branch (storage only)
                   └─ pages job ───────► dispatches pages.yml on main

pages.yml ──► landing page (site/, Astro + VHS recordings)
              + APT files from gh-pages ──► GitHub Pages
                                            https://camilo-ovalle.github.io/lazynet
```

The landing page and the APT repository share one GitHub Pages site.
pages.yml builds the site, copies the APT files from `gh-pages` next to it,
checks that `InRelease` still verifies, and deploys the result. It also runs
on its own whenever `site/` or `assets/` change on `main`.

An APT repository is a set of static files served over HTTPS, so GitHub Pages
can host it. `scripts/build-apt-repo.sh` generates them:

| File | Purpose |
|---|---|
| `pool/main/l/lazynet/*.deb` | The packages. Older versions are kept. |
| `dists/stable/main/binary-<arch>/Packages(.gz)` | Index: one entry per `.deb`, with its version, path and SHA-256. |
| `dists/stable/Release` | Hashes of every index file. |
| `dists/stable/InRelease`, `Release.gpg` | `Release` signed with the repository key. |
| `lazynet.gpg` | Public key that users save to `/usr/share/keyrings/`. |

When a user runs `apt update`, apt checks the chain of trust in this order:

1. The public key in `/usr/share/keyrings/lazynet.gpg` verifies the signature of `InRelease`.
2. The hashes inside `InRelease` verify the `Packages` index.
3. The SHA-256 in `Packages` verifies the `.deb` that `apt install` downloads.

If anything is altered in between, apt refuses it. `signed-by=` limits the key
to this repository only, so it can't vouch for packages from other sources.

## One-time setup

### 1. Create the signing key

Use a dedicated key that is only for signing the repository. Don't use your
personal GPG key.

```sh
gpg --quick-gen-key "lazynet APT repository" rsa4096 sign never
gpg --list-secret-keys --keyid-format long "lazynet APT repository"
```

gpg asks for a passphrase. The key does not expire (`never`). If it expired,
`apt update` would fail for every user until they downloaded the key again.
gpg also writes a revocation certificate to `~/.gnupg/openpgp-revocs.d/`.

**Back up** the private key and the passphrase in a password manager:

```sh
gpg --armor --export-secret-keys "lazynet APT repository" > lazynet-apt-private.asc
```

If you lose the key, you must create a new one, and every user has to
download the new public key.

### 2. Add the repository secrets

Go to GitHub, then *Settings → Secrets and variables → Actions → New repository secret*:

| Name | Value |
|---|---|
| `APT_GPG_PRIVATE_KEY` | Full contents of `lazynet-apt-private.asc`, including the `-----BEGIN/END PGP PRIVATE KEY BLOCK-----` lines |
| `APT_GPG_PASSPHRASE` | The key's passphrase |

Afterwards, delete the local `lazynet-apt-private.asc` file, or move it to
your backup.

### 3. Enable GitHub Pages

Go to *Settings → Pages → Build and deployment → Source* and select
**GitHub Actions**. `gh-pages` stays as the storage branch for the APT
repository, but GitHub no longer serves it directly; pages.yml deploys it
together with the landing page.

## Cutting a release

1. Make sure the CI is green on `main`. The `packaging` job has already built
   every package, built an APT repository and installed lazynet from it.
2. Tag the release and push the tag (semantic versioning: `vMAJOR.MINOR.PATCH`):

   ```sh
   git switch main && git pull
   git tag -a v0.1.0 -m "lazynet v0.1.0"
   git push origin v0.1.0
   ```

3. Watch the run in *Actions → release*. When it finishes:
   - *Releases* shows the tarballs, `.deb`, `.rpm` and `checksums.txt`;
   - `gh-pages` contains the updated APT repository.

## Republishing the APT repository

If the `apt-repo` job fails after the GitHub Release was created, there's no
need to tag again. Go to *Actions → release → Run workflow*, keep the `main`
branch selected, and enter the existing tag (for example `v0.1.0`). This job
downloads the `.deb` files from that release, rebuilds the repository and
publishes it. Re-running the failed run instead would reuse the workflow
file from the tagged commit, including any bug that has since been fixed.

## Verifying a release

Run this on a clean Debian or Ubuntu machine, VM or container:

```sh
curl -fsSL https://camilo-ovalle.github.io/lazynet/lazynet.gpg | sudo tee /usr/share/keyrings/lazynet.gpg >/dev/null
echo "deb [signed-by=/usr/share/keyrings/lazynet.gpg] https://camilo-ovalle.github.io/lazynet stable main" | sudo tee /etc/apt/sources.list.d/lazynet.list
sudo apt update && sudo apt install lazynet
lazynet version
```

```sh
curl -fsSL https://raw.githubusercontent.com/Camilo-ovalle/lazynet/main/scripts/install.sh | sh
```

## Testing the repository locally

`scripts/build-apt-repo.sh` runs the same way locally and in CI. Use a
throwaway key in a temporary `GNUPGHOME`, so your real keyring stays
untouched:

```sh
export GNUPGHOME=$(mktemp -d)
gpg --batch --passphrase '' --quick-gen-key "test <test@invalid>" rsa3072 sign never
KEY=$(gpg --list-secret-keys --with-colons | awk -F: '/^fpr/{print $10; exit}')
goreleaser release --snapshot --clean          # or any folder with .deb files
APT_GPG_KEY_ID=$KEY sh scripts/build-apt-repo.sh dist /tmp/apt-repo
python3 -m http.server 8080 --directory /tmp/apt-repo
```

Then point an apt source at `http://127.0.0.1:8080 stable main`, with
`signed-by=/tmp/apt-repo/lazynet.gpg`.
