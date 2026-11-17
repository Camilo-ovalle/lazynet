#!/bin/sh
# Copyright 2026 Camilo Enrique Ovalle Ceron
# SPDX-License-Identifier: Apache-2.0
#
# Prints the CHANGELOG.md section of a release, used as its GitHub release
# notes by release.yml. Fails when the version has no section, so a release
# can't be published without notes (the TUI shows the same text after an
# update, from the copy embedded in the binary).
#
#   sh scripts/release-notes.sh v0.2.0 [CHANGELOG.md] > notes.md
set -eu

die() { printf 'release-notes: %s\n' "$*" >&2; exit 1; }

[ $# -ge 1 ] || die "usage: release-notes.sh vX.Y.Z [CHANGELOG.md]"
version=${1#v}
file=${2:-CHANGELOG.md}
[ -r "$file" ] || die "cannot read $file"

notes=$(awk -v v="$version" '
  /^## / {
    if (found) exit
    h = "## [" v "]"
    rest = substr($0, length(h) + 1)
    if (substr($0, 1, length(h)) == h && (rest == "" || substr(rest, 1, 1) == " ")) { found = 1; next }
  }
  found && !started && /^[[:space:]]*$/ { next }
  found { started = 1; print }
' "$file")

[ -n "$notes" ] || die "$file has no section '## [$version] - YYYY-MM-DD': add the release notes before tagging (see docs/releasing.md)"

printf '%s\n\n' "$notes"
printf '%s\n' "Already using lazynet? Run \`lazynet update\`. New here? See the [installation guide](https://github.com/Camilo-ovalle/lazynet#installation)."
