// Command lazynet is a lazy-style TUI and CLI for Cisco network devices.
package main

import (
	"os"

	"github.com/Camilo-ovalle/Lazy_Net/internal/cli"
)

func main() { os.Exit(cli.Execute()) }
