#!/bin/sh
# lazynet installer — downloads the latest release binary for your CPU,
# verifies its SHA-256 checksum and installs it. No other dependencies.
#
#   curl -fsSL https://raw.githubusercontent.com/Camilo-ovalle/Lazy_Net/main/scripts/install.sh | sh
#
# Environment overrides:
#   LAZYNET_VERSION      version to install (e.g. 0.1.0), default: latest
#   LAZYNET_INSTALL_DIR  target directory, default: /usr/local/bin (or ~/.local/bin)
#   LAZYNET_BASE_URL     download mirror (expects GitHub release layout)
set -eu

REPO="Camilo-ovalle/Lazy_Net"
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

  dir="${LAZYNET_INSTALL_DIR:-}"
  sudo=""
  if [ -z "$dir" ]; then
    if [ -w /usr/local/bin ]; then
      dir=/usr/local/bin
    elif command -v sudo >/dev/null 2>&1 && { [ -t 0 ] || [ -t 1 ]; }; then
      dir=/usr/local/bin
      sudo="sudo"
      say "Installing to /usr/local/bin (sudo may ask for your password)..."
    else
      dir="$HOME/.local/bin"
    fi
  fi
  $sudo mkdir -p "$dir"
  $sudo install -m 0755 "$tmp/lazynet" "$dir/lazynet" 2>/dev/null || $sudo cp "$tmp/lazynet" "$dir/lazynet"

  say "lazynet $version installed to $dir/lazynet"
  case ":$PATH:" in
    *":$dir:"*) ;;
    *) say "Note: $dir is not in your PATH. Add it with:"
       say "  echo 'export PATH=\"$dir:\$PATH\"' >> ~/.bashrc  # or ~/.zshrc" ;;
  esac
  say "Run 'lazynet --demo' to try it with emulated devices, or 'lazynet inventory init' to start."
}

main "$@"
