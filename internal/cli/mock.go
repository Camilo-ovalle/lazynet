// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/Camilo-ovalle/lazynet/internal/mockssh"
	"github.com/spf13/cobra"
)

// newMockCmd runs an emulated device for lab testing (hidden).
func newMockCmd() *cobra.Command {
	var listen, platform, host, user, pass string
	var unpriv bool
	c := &cobra.Command{
		Use:    "mock",
		Short:  "Run an emulated Cisco device over SSH (for testing)",
		Hidden: true,
		RunE: func(*cobra.Command, []string) error {
			dev := mockssh.NewDevice(platform, host)
			dev.Unprivileged = unpriv
			srv, err := mockssh.Start(listen, dev, user, pass)
			if err != nil {
				return err
			}
			defer srv.Close()
			fmt.Printf("emulated %s %q listening on %s (user %q, password %q). Ctrl+C to stop.\n", platform, host, srv.Addr(), user, pass)
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, os.Interrupt)
			<-sig
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&listen, "listen", "127.0.0.1:2222", "listen address")
	f.StringVar(&platform, "platform", "catalyst", "catalyst, nexus or asa")
	f.StringVar(&host, "hostname", "mock-sw1", "device hostname")
	f.StringVar(&user, "user", "admin", "accepted username")
	f.StringVar(&pass, "password", "admin", "accepted password")
	f.BoolVar(&unpriv, "unprivileged", false, "log in at user EXEC (>)")
	return c
}
