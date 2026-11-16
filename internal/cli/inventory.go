// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/Camilo-ovalle/lazynet/internal/render"
	"net"
	"os"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/inventory"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/spf13/cobra"
)

func newInventoryCmd(g *globals) *cobra.Command {
	c := &cobra.Command{Use: "inventory", Aliases: []string{"inv"}, Short: "Manage the device inventory"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List inventory devices",
		RunE: func(*cobra.Command, []string) error {
			inv := g.app.Inv
			if g.json() {
				return printJSON(inv.Devices)
			}
			if len(inv.Devices) == 0 {
				return fmt.Errorf("inventory is empty: run 'lazynet inventory init' or 'lazynet inventory add' (%s)", inv.Path())
			}
			det := g.app.DetectedPlatforms()
			t := newTable().Headers("NAME", "HOST", "GROUP", "USER", "TYPE", "DESCRIPTION")
			for _, d := range inv.Devices {
				p, _ := platform.Parse(d.Platform)
				if p == platform.Unknown {
					p = det[d.Label()]
				}
				t.Row(d.Label(), d.Addr(), d.Group, d.Username, render.PlatformBadge(p), d.Description)
			}
			fmt.Println(t)
			return nil
		},
	}
	var nd inventory.Device
	add := &cobra.Command{
		Use:     "add --name <name> --host <ip>",
		Short:   "Add a device to the inventory",
		Example: `  lazynet inventory add --name core-sw1 --host 192.168.100.100 --group core --username admin`,
		RunE: func(*cobra.Command, []string) error {
			if nd.Host == "" {
				return errors.New("--host is required")
			}
			if _, err := platform.Parse(nd.Platform); err != nil {
				return err
			}
			if g.app.Demo {
				return errors.New("inventory changes are disabled in demo mode")
			}
			if err := inventory.Add(g.app.Paths.Inventory, nd); err != nil {
				return err
			}
			fmt.Println(theme.Good.Render("✓ added " + nd.Label() + " to " + g.app.Paths.Inventory))
			return nil
		},
	}
	af := add.Flags()
	af.StringVar(&nd.Name, "name", "", "device name")
	af.StringVar(&nd.Host, "host", "", "IP address or hostname")
	af.IntVar(&nd.Port, "port", 0, "SSH port (default 22)")
	af.StringVar(&nd.Group, "group", "", "group name")
	af.StringVar(&nd.Username, "username", "", "SSH username")
	af.StringVar(&nd.Platform, "platform", "", "pin platform: "+platform.Names()+" (default: auto-detect)")
	af.StringVar(&nd.Description, "description", "", "free text")

	var reach bool
	check := &cobra.Command{
		Use:   "check",
		Short: "Validate the inventory file (and optionally test TCP reachability)",
		RunE: func(*cobra.Command, []string) error {
			inv := g.app.Inv
			errs := inv.Validate(func(p string) error { _, err := platform.Parse(p); return err })
			for _, e := range errs {
				fmt.Println(theme.Bad.Render("✗ " + e.Error()))
			}
			if len(errs) == 0 {
				fmt.Println(theme.Good.Render(fmt.Sprintf("✓ %d devices, %d groups, no problems", len(inv.Devices), len(inv.Groups()))))
			}
			if reach {
				res := g.app.ForEach(inv.Devices, func(d inventory.Device) (any, error) {
					start := time.Now()
					c, err := net.DialTimeout("tcp", d.Addr(), g.app.Cfg.SSHTimeout.D())
					if err != nil {
						return nil, err
					}
					_ = c.Close()
					return time.Since(start).Round(time.Millisecond), nil
				})
				for _, r := range res {
					if r.Err != nil {
						fmt.Printf("%s %-20s %s\n", theme.Bad.Render("✗"), r.Device.Label(), theme.Bad.Render(r.Err.Error()))
						errs = append(errs, r.Err)
					} else {
						fmt.Printf("%s %-20s ssh port open (%v)\n", theme.Good.Render("✓"), r.Device.Label(), r.Value)
					}
				}
			}
			if len(errs) > 0 {
				return fmt.Errorf("%d problem(s) found", len(errs))
			}
			return nil
		},
	}
	check.Flags().BoolVar(&reach, "reach", false, "test TCP connectivity to each device's SSH port")

	init := &cobra.Command{
		Use:   "init",
		Short: "Create an example inventory file",
		RunE: func(*cobra.Command, []string) error {
			p := g.app.Paths.Inventory
			if _, err := os.Stat(p); err == nil {
				return fmt.Errorf("%s already exists", p)
			}
			if err := os.WriteFile(p, []byte(inventory.Example), 0o600); err != nil {
				return err
			}
			fmt.Println(theme.Good.Render("✓ created "+p) + theme.Dim.Render(" (edit it with your devices)"))
			return nil
		},
	}
	path := &cobra.Command{
		Use:   "path",
		Short: "Print where lazynet keeps its files",
		Run: func(*cobra.Command, []string) {
			p := g.app.Paths
			fmt.Println(render.KV([][2]string{
				{"Inventory", p.Inventory}, {"Config", p.Config}, {"Known hosts", p.KnownHosts},
				{"Backups", p.Backups}, {"Audit log", p.AuditLog},
				{"Password cache", fmt.Sprintf("%s, %v grace period", g.app.Creds.Backend(), g.app.Cfg.CredentialTTL.D())},
			}))
		},
	}
	c.AddCommand(list, add, check, init, path)
	return c
}
