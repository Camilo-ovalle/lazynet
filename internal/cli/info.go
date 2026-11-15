// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"github.com/Camilo-ovalle/lazynet/internal/render"
	"os"

	"github.com/Camilo-ovalle/lazynet/internal/device"
	"github.com/Camilo-ovalle/lazynet/internal/inventory"
	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

func newInfoCmd(g *globals) *cobra.Command {
	c := &cobra.Command{
		Use:   "info <device>",
		Short: "Show serial, MAC, IP, OS version and uptime",
		Example: `  lazynet info core-sw1
  lazynet info 192.168.100.100 -u admin
  lazynet info --group core -o json`,
		RunE: func(_ *cobra.Command, args []string) error {
			devs, err := g.targets(args)
			if err != nil {
				return err
			}
			res := g.app.ForEach(devs, func(d inventory.Device) (any, error) {
				s, err := g.connect(d)
				if err != nil {
					return nil, err
				}
				defer s.Close()
				return s.Driver.Info(d.Host)
			})
			if g.json() {
				out := make([]map[string]any, len(res))
				for i, r := range res {
					out[i] = map[string]any{"device": r.Device.Label(), "info": r.Value}
					if r.Err != nil {
						out[i]["error"] = r.Err.Error()
					}
				}
				if len(out) == 1 && res[0].Err == nil {
					return printJSON(res[0].Value)
				}
				return printJSON(out)
			}
			if len(res) == 1 {
				if res[0].Err != nil {
					return res[0].Err
				}
				fmt.Println(render.Info(res[0].Device.Label(), res[0].Value.(device.Info)))
				return nil
			}
			return infoTable(res)
		},
	}
	c.Flags().StringVarP(&g.group, "group", "g", "", "run on every device of an inventory group")
	return c
}

func newTable() *table.Table {
	return table.New().Border(lipgloss.RoundedBorder()).BorderStyle(lipgloss.NewStyle().Foreground(theme.Subtle)).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return theme.Header.Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})
}

func infoTable(res []resultT) error {
	t := newTable().Headers("DEVICE", "TYPE", "MODEL", "SERIAL", "MAC", "OS", "UPTIME")
	failed := 0
	for _, r := range res {
		if r.Err != nil {
			failed++
			t.Row(r.Device.Label(), theme.Bad.Render("error"), theme.Bad.Render(trim(r.Err.Error(), 60)), "", "", "", "")
			continue
		}
		i := r.Value.(device.Info)
		t.Row(r.Device.Label(), theme.PlatformBadge(string(i.Platform)), i.Model, i.Serial, i.MAC, i.OS+" "+i.Version, i.Uptime)
	}
	fmt.Println(t)
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d of %d devices failed\n", failed, len(res))
		return fmt.Errorf("%d device(s) failed", failed)
	}
	return nil
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
