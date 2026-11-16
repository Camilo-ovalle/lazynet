#!/bin/sh
# Copyright 2026 Camilo Enrique Ovalle Ceron
# SPDX-License-Identifier: Apache-2.0
#
# Builds or updates a signed APT repository from .deb packages.
#
#   APT_GPG_KEY_ID=<key> scripts/build-apt-repo.sh <debs-dir> <repo-dir>
#
# <repo-dir> may already contain older releases (the gh-pages branch).
# Their packages are kept, so users can still install or pin old versions.
#
# Environment:
#   APT_GPG_KEY_ID      signing key, already imported into gpg (required)
#   APT_GPG_PASSPHRASE  passphrase of that key (optional)
#   APT_SUITE           distribution name users put in sources.list (stable)
#   APT_COMPONENT       component name (main)
#   APT_ARCHS           architectures to index ("amd64 arm64")
#
# Resulting layout, the standard "dists" layout that apt expects:
#
#   pool/main/l/lazynet/lazynet_<ver>_<arch>.deb   the packages themselves
#   dists/stable/main/binary-<arch>/Packages(.gz)  index: one stanza per .deb
#                                                  with its version, deps,
#                                                  path and SHA-256
#   dists/stable/Release                           hashes of every index file
#   dists/stable/InRelease                         Release, clear-signed
#   dists/stable/Release.gpg                       detached signature (older apt)
#   lazynet.gpg / lazynet.asc                      public key users install
#
# Chain of trust used by `apt update` / `apt install`:
#   public key -> verifies InRelease -> its hashes verify Packages
#   -> the SHA-256 in Packages verifies the downloaded .deb
set -eu

[ $# -eq 2 ] || { echo "usage: APT_GPG_KEY_ID=<key> $0 <debs-dir> <repo-dir>" >&2; exit 2; }
debs=$(cd "$1" && pwd)
repo=$2
: "${APT_GPG_KEY_ID:?set APT_GPG_KEY_ID to the signing key}"
suite=${APT_SUITE:-stable}
comp=${APT_COMPONENT:-main}
archs=${APT_ARCHS:-amd64 arm64}

set -- "$debs"/*.deb
[ -e "$1" ] || { echo "no .deb files in $debs" >&2; exit 1; }

# 1. Pool: the .deb files. Debian convention is pool/<component>/<first
#    letter>/<source package>/.
mkdir -p "$repo/pool/$comp/l/lazynet"
cp "$@" "$repo/pool/$comp/l/lazynet/"
cd "$repo"

dist="dists/$suite"
# Drop the previous signatures and Release. Release lists the hash of every
# file under dists/<suite>, so a stale Release must not be hashed into the
# new one.
rm -f "$dist/Release" "$dist/Release.gpg" "$dist/InRelease"

# 2. Packages indexes, one per architecture. Each stanza is the package's
#    control file plus Filename, Size and checksums. Every version in the
#    pool is listed, and apt installs the highest one.
for arch in $archs; do
  dir="$dist/$comp/binary-$arch"
  mkdir -p "$dir"
  apt-ftparchive --arch "$arch" packages "pool/$comp" > "$dir/Packages"
  gzip -9nkf "$dir/Packages"
done

# 3. Release: suite metadata plus the hashes of the Packages files. It is
#    written outside dists/ first, so it never lists itself.
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
apt-ftparchive \
  -o APT::FTPArchive::Release::Origin=lazynet \
  -o APT::FTPArchive::Release::Label=lazynet \
  -o APT::FTPArchive::Release::Suite="$suite" \
  -o APT::FTPArchive::Release::Codename="$suite" \
  -o APT::FTPArchive::Release::Architectures="$archs" \
  -o APT::FTPArchive::Release::Components="$comp" \
  -o APT::FTPArchive::Release::Description="lazynet: TUI and CLI for Cisco Catalyst, Nexus and ASA" \
  release "$dist" > "$tmp"
mv "$tmp" "$dist/Release"
trap - EXIT

# 4. Signatures. InRelease (Release with an inline signature) is what modern
#    apt reads. Release.gpg is the detached signature for older clients.
sign() {
  printf '%s' "${APT_GPG_PASSPHRASE:-}" |
    gpg --batch --yes --pinentry-mode loopback --passphrase-fd 0 \
      --local-user "$APT_GPG_KEY_ID" --digest-algo SHA512 "$@"
}
sign --armor --detach-sign --output "$dist/Release.gpg" "$dist/Release"
sign --clearsign --output "$dist/InRelease" "$dist/Release"

# 5. Public key. lazynet.gpg (binary) goes in /usr/share/keyrings and is
#    referenced with signed-by=. lazynet.asc is the same key, readable.
gpg --batch --yes --export "$APT_GPG_KEY_ID" > lazynet.gpg
gpg --batch --yes --armor --export "$APT_GPG_KEY_ID" > lazynet.asc

echo "APT repository ready in $(pwd): suite=$suite component=$comp archs=$archs"
ls "pool/$comp/l/lazynet"
