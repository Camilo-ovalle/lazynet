#!/bin/sh
# Copyright 2026 Camilo Enrique Ovalle Ceron
# SPDX-License-Identifier: Apache-2.0
#
# lazynet installer — downloads the latest release binary for your CPU,
# verifies its SHA-256 checksum and installs it. No other dependencies.
# Run it again to update: it replaces an existing lazynet where it is.
# `lazynet update` does the same from lazynet itself.
#
#   curl -fsSL https://raw.githubusercontent.com/Camilo-ovalle/lazynet/main/scripts/install.sh | sh
#
# Environment overrides:
#   LAZYNET_VERSION      version to install (e.g. 0.1.0), default: latest
#   LAZYNET_INSTALL_DIR  target directory, default: where lazynet already is,
#                        otherwise /usr/local/bin (or ~/.local/bin)
#   LAZYNET_BASE_URL     download mirror (expects GitHub release layout)
set -eu

REPO="Camilo-ovalle/lazynet"
BASE_URL="${LAZYNET_BASE_URL:-https://github.com/$REPO/releases}"

say() { printf '%s\n' "$*"; }
die() { printf 'lazynet install: %s\n' "$*" >&2; exit 1; }

need_fetcher() {
  if command -v curl >/dev/null 2>&1; then FETCH="curl"; return; fi
  if command -v wget >/dev/null 2>&1; then FETCH="wget"; return; fi
  die "curl or wget is required"
}

fetch() { # url dest
  if [ "$FETCH" = curl ]; then curl -fsSL "$1" -o "$2"; else wget -qO "$2" "$1"; fi
}

latest_version() {
  if [ "$FETCH" = curl ]; then
    url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$BASE_URL/latest")
  else
    url=$(wget -S --spider "$BASE_URL/latest" 2>&1 | sed -n 's/^ *Location: *//p' | tail -n1)
  fi
  v=${url##*/}
  v=${v#v}
  [ -n "$v" ] && [ "$v" != latest ] || die "could not determine the latest version"
  printf '%s' "$v"
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo amd64 ;;
    aarch64 | arm64) echo arm64 ;;
    *) die "unsupported CPU architecture: $(uname -m) (supported: x86_64, arm64)" ;;
  esac
}

# installed prints the path of the lazynet found in PATH (symlinks
# resolved), or nothing.
installed() {
  p=$(command -v lazynet 2>/dev/null) || return 0
  case $p in /*) ;; *) return 0 ;; esac # an alias or function, not a file
  readlink -f "$p" 2>/dev/null || printf '%s\n' "$p"
}

# writable reports whether dir, or the parent it would be created in, is
# writable by the current user.
writable() {
  d=$1
  while [ ! -d "$d" ]; do d=$(dirname "$d"); done
  [ -w "$d" ]
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then shasum -a 256 "$1" | cut -d' ' -f1
  else die "sha256sum or shasum is required to verify the download"; fi
}

main() {
  [ "$(uname -s)" = Linux ] || die "this installer supports Linux only for now"
  need_fetcher
  arch=$(detect_arch)
  version="${LAZYNET_VERSION:-$(latest_version)}"
  version=${version#v}
  file="lazynet_${version}_linux_${arch}.tar.gz"

  # An existing install is updated in place. The .deb and .rpm packages own
  # /usr/bin/lazynet: leave it to them and install next to it instead.
  existing=$(installed)
  current=""
  [ -z "$existing" ] || current=$("$existing" version 2>/dev/null | awk 'NR == 1 { print $2 }') || current=""
  dir="${LAZYNET_INSTALL_DIR:-}"
  if [ -z "$dir" ] && [ -n "$existing" ] && [ "$existing" != /usr/bin/lazynet ]; then
    dir=$(dirname "$existing")
  fi
  if [ "$current" = "$version" ] && [ "$existing" = "${dir:-}/lazynet" ]; then
    say "lazynet $version is already installed in $dir: nothing to do."
    return 0
  fi

  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT INT TERM

  say "Downloading lazynet $version ($arch)..."
  fetch "$BASE_URL/download/v$version/$file" "$tmp/$file" || die "download failed: $BASE_URL/download/v$version/$file"
  fetch "$BASE_URL/download/v$version/checksums.txt" "$tmp/checksums.txt" || die "could not download checksums.txt"

  want=$(grep " $file\$" "$tmp/checksums.txt" | cut -d' ' -f1)
  [ -n "$want" ] || die "$file is not listed in checksums.txt"
  got=$(sha256 "$tmp/$file")
  [ "$want" = "$got" ] || die "checksum mismatch for $file (expected $want, got $got)"

  tar -xzf "$tmp/$file" -C "$tmp" lazynet || die "could not extract the archive"
  chmod 0755 "$tmp/lazynet"

  sudo=""
  if [ -z "$dir" ]; then
    if [ -w /usr/local/bin ]; then
      dir=/usr/local/bin
    elif command -v sudo >/dev/null 2>&1 && { [ -t 0 ] || [ -t 1 ]; }; then
      dir=/usr/local/bin
    else
      dir="$HOME/.local/bin"
    fi
  fi
  if ! writable "$dir"; then
    command -v sudo >/dev/null 2>&1 || die "$dir is not writable and sudo is not available: run as root or set LAZYNET_INSTALL_DIR"
    sudo="sudo"
    say "Installing to $dir (sudo may ask for your password)..."
  fi
  $sudo mkdir -p "$dir"
  $sudo install -m 0755 "$tmp/lazynet" "$dir/lazynet" 2>/dev/null || $sudo cp "$tmp/lazynet" "$dir/lazynet"

  if [ "$existing" = "$dir/lazynet" ]; then
    # Updated in place: PATH already finds it (maybe through a symlink).
    say "lazynet updated from ${current:-an unknown version} to $version in $dir/lazynet"
    return 0
  fi
  say "lazynet $version installed to $dir/lazynet"
  if [ "$existing" = /usr/bin/lazynet ] && [ "$dir" != /usr/bin ]; then
    say "Note: a lazynet package is also installed in /usr/bin. Remove it to keep one copy:"
    say "  sudo apt remove lazynet    # or: sudo dnf remove lazynet"
  fi
  case ":$PATH:" in
    *":$dir:"*) ;;
    *) say "Note: $dir is not in your PATH. Add it with:"
       say "  echo 'export PATH=\"$dir:\$PATH\"' >> ~/.bashrc  # or ~/.zshrc" ;;
  esac
  say "Run 'lazynet --demo' to try it with emulated devices, or 'lazynet inventory init' to start."
  say "Update later with 'lazynet update'."
}

main "$@"
