// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package cli implements the lazynet command line (cobra). Running lazynet
// without a subcommand starts the TUI.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/Camilo-ovalle/lazynet/internal/app"
	"github.com/Camilo-ovalle/lazynet/internal/config"
	"github.com/Camilo-ovalle/lazynet/internal/inventory"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/Camilo-ovalle/lazynet/internal/tui"
	"github.com/spf13/cobra"
)

// Build information, set via -ldflags by the release pipeline.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

type globals struct {
	demo    bool
	output  string
	user    string
	sshPort int
	group   string

	app     *app.App
	cleanup func()
	prompt  *termPrompter
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	g := &globals{prompt: newPrompter()}
	root := newRoot(g)
	err := root.Execute()
	if g.cleanup != nil {
		g.cleanup()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, theme.Bad.Render("error: ")+err.Error())
		return 1
	}
	return 0
}

func newRoot(g *globals) *cobra.Command {
	root := &cobra.Command{
		Use:   "lazynet",
		Short: "A lazy terminal UI and CLI for " + platform.Describe(platform.All()),
		Long: `lazynet talks to network devices over SSH: ` + platform.Describe(platform.All()) + `.

Run it without arguments for the interactive TUI, or use subcommands:

  lazynet info core-sw1
  lazynet iface 192.168.100.100 --port Gi1/0/25
  lazynet vlan set core-sw1 --port 1/0/1 --vlan 110
  lazynet logs core-sw1 --from "2026-11-01 08:00" --to "2026-11-01 18:00"
  lazynet backup --group core

Everything is read-only ('show' commands) except 'vlan set', which always
shows the exact commands and asks for confirmation before applying them.
Devices can be inventory names or IP addresses. Try --demo to play with
emulated devices without touching the network.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			for c := cmd; c != nil; c = c.Parent() {
				switch c.Name() {
				case "version", "help", "mock", "completion", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
					return nil // must not touch the user's files
				}
			}
			return g.load()
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tui.Run(g.app)
		},
	}
	pf := root.PersistentFlags()
	pf.BoolVar(&g.demo, "demo", false, "use emulated devices (no network, nothing is changed)")
	pf.StringVarP(&g.output, "output", "o", "table", "output format: table or json")
	pf.StringVarP(&g.user, "user", "u", "", "SSH username (overrides the inventory)")
	pf.IntVar(&g.sshPort, "ssh-port", 0, "SSH port (default 22 or the inventory value)")
	root.AddCommand(
		newInfoCmd(g), newIfaceCmd(g), newVlanCmd(g), newLogsCmd(g),
		newBackupCmd(g), newInventoryCmd(g), newVersionCmd(), newMockCmd(),
	)
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if strings.Contains(c.Use, "<device>") || strings.Contains(c.Use, "[device]") {
			c.ValidArgsFunction = completeDevices
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
	return root
}

// completeDevices offers inventory names for shell completion without
// creating any files.
func completeDevices(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	p, err := config.DefaultPaths()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	inv, err := inventory.Load(p.Inventory)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var out []string
	for _, d := range inv.Devices {
		out = append(out, d.Label()+"\t"+d.Host+" "+d.Group)
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

func (g *globals) load() error {
	if g.output != "table" && g.output != "json" {
		return fmt.Errorf("--output must be 'table' or 'json'")
	}
	var err error
	if g.demo {
		g.app, g.cleanup, err = app.StartDemo()
		if err == nil && g.output == "table" {
			fmt.Fprintln(os.Stderr, theme.Caution.Render("demo mode: emulated devices "+strings.Join(names(g.app.Inv.Devices), ", ")))
		}
		return err
	}
	g.app, err = app.Load()
	return err
}

func names(ds []inventory.Device) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Label()
	}
	return out
}

// device resolves a target and applies flag overrides.
func (g *globals) device(target string) inventory.Device {
	d := g.app.Inv.Resolve(target)
	if g.user != "" {
		d.Username = g.user
	}
	if g.sshPort != 0 {
		d.Port = g.sshPort
	}
	return d
}

// targets returns the devices selected by args or --group.
func (g *globals) targets(args []string) ([]inventory.Device, error) {
	if g.group != "" {
		if len(args) > 0 {
			return nil, errors.New("use either a device or --group, not both")
		}
		ds := g.app.Inv.Group(g.group)
		if len(ds) == 0 {
			return nil, fmt.Errorf("group %q has no devices in %s", g.group, g.app.Inv.Path())
		}
		for i := range ds {
			if g.user != "" {
				ds[i].Username = g.user
			}
		}
		return ds, nil
	}
	if len(args) != 1 {
		return nil, errors.New("specify one device (inventory name or IP) or --group")
	}
	return []inventory.Device{g.device(args[0])}, nil
}

func (g *globals) connect(d inventory.Device) (*app.Session, error) {
	if g.output == "table" {
		fmt.Fprintln(os.Stderr, theme.Dim.Render("connecting to "+d.Label()+" ("+d.Addr()+")..."))
	}
	return g.app.Connect(d, g.prompt)
}

func (g *globals) json() bool { return g.output == "json" }

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the lazynet version",
		Run: func(*cobra.Command, []string) {
			v, c, d := buildInfo()
			fmt.Printf("lazynet %s (commit %s, built %s)\n", v, c, d)
		},
	}
}

// buildInfo returns the version, commit and build date. Release builds get
// them from -ldflags. Otherwise they come from the information the Go
// toolchain embeds:
// - `go install …@v0.1.0` records the module version;
// - a `go build` inside a git checkout records the revision and its time.
func buildInfo() (version, commit, date string) {
	version, commit, date = Version, Commit, Date
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = strings.TrimPrefix(bi.Main.Version, "v")
	}
	for _, s := range bi.Settings {
		switch {
		case s.Key == "vcs.revision" && commit == "none":
			commit = s.Value
			if len(commit) > 7 {
				commit = commit[:7]
			}
		case s.Key == "vcs.time" && date == "unknown":
			date = s.Value
		}
	}
	return
}
