# Copyright 2026 Camilo Enrique Ovalle Ceron
# SPDX-License-Identifier: Apache-2.0
#
# Hero recording: browsing the TUI in demo mode (emulated devices).
# Run from the repository root:  vhs site/tapes/tui.tape
Output site/public/media/tui.webm
Output site/public/media/tui.mp4
Require lazynet
Source site/tapes/settings.tape

# Start off camera: the TUI waits briefly for the terminal colour query.
Hide
Type "lazynet --demo"
Enter
Sleep 6s
Show

Sleep 1.2s
# access-sw2 -> core-sw1, connect
Type "j"
Sleep 700ms
Enter
Sleep 2.5s
# Interface tab: query a port
Type "2"
Sleep 900ms
Type@90ms "1/0/25"
Sleep 500ms
Enter
Sleep 2.8s
# Logs tab, then only warnings and worse
Type "3"
Sleep 2.2s
Type "s"
Sleep 2.4s
# Backups tab: take a backup
Type "4"
Sleep 1s
Type "b"
Sleep 2.6s
# Another device, back to Info
Type "j"
Sleep 600ms
Enter
Sleep 1s
Type "1"
Sleep 2.5s

Hide
Type "q"
