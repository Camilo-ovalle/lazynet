# Copyright 2026 Camilo Enrique Ovalle Ceron
# SPDX-License-Identifier: Apache-2.0
#
# VLAN change in demo mode: form, exact commands, explicit y, save prompt.
# Run from the repository root:  vhs site/tapes/vlan.tape
Output site/public/media/vlan.webm
Output site/public/media/vlan.mp4
Require lazynet
Source site/tapes/settings.tape

Hide
Type "lazynet --demo"
Enter
Sleep 6s
# select and connect to core-sw1 off camera
Type "j"
Sleep 500ms
Enter
Sleep 2s
Show

Sleep 1.2s
Type "v"
Sleep 900ms
Type@110ms "1/0/7"
Sleep 400ms
Tab
Sleep 300ms
Type@110ms "20"
Sleep 600ms
Enter
# read-only checks, then the exact commands
Sleep 4.5s
Type "y"
Sleep 2.5s
# separate question before write memory
Type "y"
Sleep 3s

Hide
Type "q"
