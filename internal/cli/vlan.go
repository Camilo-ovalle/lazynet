// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/Camilo-ovalle/lazynet/internal/render"

	"github.com/Camilo-ovalle/lazynet/internal/device"
	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/spf13/cobra"
)

func newVlanCmd(g *globals) *cobra.Command {
	vlan := &cobra.Command{Use: "vlan", Short: "VLAN operations (Catalyst)"}
	var port string
	var id int
	set := &cobra.Command{
		Use:   "set <device> --port <interface> --vlan <id>",
		Short: "Change the access VLAN of a port (asks for confirmation)",
		Long: `Change the access VLAN of a switchport on a Catalyst switch.

Before anything is sent, lazynet checks (with show commands only) that the
port is an access port (trunks are refused), that the VLAN exists and is
active, then prints the exact commands and asks for confirmation. After the
change it reads the port back to verify it, and asks separately whether to
save the configuration with 'write memory'. Every attempt is recorded in the
audit log.`,
		Example: `  lazynet vlan set core-sw1 --port Gi1/0/1 --vlan 110
  lazynet vlan set 192.168.100.100 -p 1/0/1 -v 110 -u admin`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if port == "" || id == 0 {
				return errors.New("--port and --vlan are required")
			}
			if g.json() {
				return errors.New("vlan set is interactive and does not support -o json")
			}
			return g.vlanSet(args[0], port, id)
		},
	}
	set.Flags().StringVarP(&port, "port", "p", "", "access port (Gi1/0/1, 1/0/1, ...)")
	set.Flags().IntVarP(&id, "vlan", "v", 0, "target VLAN id")
	vlan.AddCommand(set)
	return vlan
}

func (g *globals) vlanSet(target, port string, id int) error {
	s, err := g.connect(g.device(target))
	if err != nil {
		return err
	}
	defer s.Close()
	fmt.Println(theme.Dim.Render("checking port and VLAN (read-only)..."))
	plan, err := g.app.PlanVlan(s, port, id)
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println(render.KV([][2]string{
		{"Device", fmt.Sprintf("%s (%s)  %s", s.Device.Label(), s.Device.Host, theme.PlatformBadge(string(s.Platform)))},
		{"Port", plan.Port},
		{"Current", fmt.Sprintf("VLAN %d %s", plan.CurrentVlan, theme.Dim.Render(plan.CurrentVlanName))},
		{"Target", fmt.Sprintf("VLAN %d %s", plan.TargetVlan, theme.Dim.Render(plan.TargetVlanName))},
	}))
	if plan.NoOp {
		fmt.Println("\n" + theme.Good.Render("Port is already in VLAN "+fmt.Sprint(id)+": nothing to do."))
		return nil
	}
	shown := plan.Change.Display()
	fmt.Println("\n" + theme.Bold.Render("The following commands will be sent to the device:"))
	fmt.Println(render.Commands(shown))
	ok, err := g.prompt.confirm("\nApply this change?")
	if err != nil {
		return err
	}
	if !ok {
		g.app.DeclineVlan(s.Device, s.User, plan)
		fmt.Println(theme.Caution.Render("Aborted: nothing was changed."))
		return nil
	}
	plan.Change.Confirm(shown)
	if err := g.app.ApplyVlan(s, plan); err != nil {
		return err
	}
	fmt.Println(theme.Good.Render(fmt.Sprintf("✓ %s is now in VLAN %d (verified on the device).", plan.Port, plan.TargetVlan)))

	save := device.SaveRequest(s.Platform)
	saveShown := save.Display()
	fmt.Println("\n" + theme.Bold.Render("Save running-config to startup-config? This will send:"))
	fmt.Println(render.Commands(saveShown))
	ok, err = g.prompt.confirm("Save configuration?")
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println(theme.Caution.Render("Not saved: the change is in running-config only and will be lost on reload."))
		return nil
	}
	save.Confirm(saveShown)
	if err := g.app.SaveConfig(s, save); err != nil {
		return err
	}
	fmt.Println(theme.Good.Render("✓ Configuration saved."))
	fmt.Println(theme.Dim.Render("audit log: " + g.app.Audit.Path()))
	return nil
}
