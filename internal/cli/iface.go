// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"github.com/Camilo-ovalle/lazynet/internal/render"

	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/spf13/cobra"
)

func newIfaceCmd(g *globals) *cobra.Command {
	var port string
	var raw bool
	c := &cobra.Command{
		Use:     "iface <device> --port <interface>",
		Aliases: []string{"interface"},
		Short:   "Show the status, VLAN, speed and errors of one interface",
		Example: `  lazynet iface core-sw1 --port Gi1/0/25
  lazynet iface core-sw1 -p 1/0/25 --raw
  lazynet iface dc-n9k-1 -p eth1/1`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if port == "" {
				return fmt.Errorf("--port is required (e.g. --port Gi1/0/25)")
			}
			s, err := g.connect(g.device(args[0]))
			if err != nil {
				return err
			}
			defer s.Close()
			d, err := s.Driver.Interface(port)
			if err != nil {
				return err
			}
			if g.json() {
				return printJSON(d)
			}
			fmt.Println(render.Interface(d))
			if raw {
				fmt.Println("\n" + theme.Dim.Render(d.Raw))
			}
			return nil
		},
	}
	c.Flags().StringVarP(&port, "port", "p", "", "interface (Gi1/0/25, 1/0/25, eth1/1, ...)")
	c.Flags().BoolVar(&raw, "raw", false, "also print the raw device output")
	return c
}
