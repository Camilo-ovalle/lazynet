// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Command lazynet is a lazy-style TUI and CLI for Cisco network devices.
package main

import (
	"os"

	"github.com/Camilo-ovalle/lazynet/internal/cli"
)

func main() { os.Exit(cli.Execute()) }
