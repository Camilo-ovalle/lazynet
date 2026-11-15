package cli

import (
	"errors"
	"fmt"
	"github.com/Camilo-ovalle/Lazy_Net/internal/render"
	"os"
	"strings"

	"github.com/Camilo-ovalle/Lazy_Net/internal/backup"
	"github.com/Camilo-ovalle/Lazy_Net/internal/inventory"
	"github.com/Camilo-ovalle/Lazy_Net/internal/theme"
	"github.com/spf13/cobra"
)

func newBackupCmd(g *globals) *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "backup [device]",
		Short: "Back up running-config into the local versioned store",
		Long: `Fetch 'show running-config' and store it in a local git repository
(~/.local/share/lazynet/backups by default). Unchanged configurations do not
create a new version. Use 'backup list' and 'backup diff' to review history.`,
		Example: `  lazynet backup core-sw1
  lazynet backup --group core
  lazynet backup --all
  lazynet backup list core-sw1
  lazynet backup diff core-sw1`,
		RunE: func(_ *cobra.Command, args []string) error {
			var devs []inventory.Device
			var err error
			if all {
				devs = g.app.Inv.Devices
				if len(devs) == 0 {
					return errors.New("inventory is empty")
				}
			} else if devs, err = g.targets(args); err != nil {
				return err
			}
			res := g.app.ForEach(devs, func(d inventory.Device) (any, error) {
				s, err := g.connect(d)
				if err != nil {
					return nil, err
				}
				defer s.Close()
				return g.app.Backup(s)
			})
			if g.json() {
				out := make([]map[string]any, len(res))
				for i, r := range res {
					out[i] = map[string]any{"device": r.Device.Label(), "result": r.Value}
					if r.Err != nil {
						out[i]["error"] = r.Err.Error()
					}
				}
				return printJSON(out)
			}
			failed := 0
			for _, r := range res {
				if r.Err != nil {
					failed++
					fmt.Printf("%s %-20s %s\n", theme.Bad.Render("✗"), r.Device.Label(), theme.Bad.Render(r.Err.Error()))
					continue
				}
				br := r.Value.(backup.Result)
				state := theme.Good.Render("new version " + br.Hash[:8])
				if !br.Changed {
					state = theme.Dim.Render("unchanged (" + br.Hash[:8] + ")")
				}
				fmt.Printf("%s %-20s %s\n", theme.Good.Render("✓"), r.Device.Label(), state)
			}
			st, _ := g.app.Backups()
			if st != nil {
				fmt.Fprintln(os.Stderr, theme.Dim.Render("store: "+st.Dir()))
			}
			if failed > 0 {
				return fmt.Errorf("%d of %d backups failed", failed, len(res))
			}
			return nil
		},
	}
	c.Flags().StringVarP(&g.group, "group", "g", "", "back up every device of an inventory group")
	c.Flags().BoolVar(&all, "all", false, "back up every device in the inventory")

	list := &cobra.Command{
		Use:   "list <device>",
		Short: "List stored backups of a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			st, err := g.app.Backups()
			if err != nil {
				return err
			}
			label := g.device(args[0]).Label()
			es, err := st.List(label)
			if err != nil {
				return err
			}
			if g.json() {
				return printJSON(es)
			}
			if len(es) == 0 {
				return fmt.Errorf("no backups for %s yet (run: lazynet backup %s)", label, args[0])
			}
			t := newTable().Headers("VERSION", "DATE")
			for _, e := range es {
				t.Row(e.Hash[:8], e.Time.Local().Format("2006-01-02 15:04:05"))
			}
			fmt.Println(t)
			return nil
		},
	}
	diff := &cobra.Command{
		Use:   "diff <device> [older] [newer]",
		Short: "Show what changed between two backups (default: last two)",
		Args:  cobra.RangeArgs(1, 3),
		RunE: func(_ *cobra.Command, args []string) error {
			st, err := g.app.Backups()
			if err != nil {
				return err
			}
			var older, newer string
			if len(args) > 1 {
				older = args[1]
			}
			if len(args) > 2 {
				newer = args[2]
			}
			d, err := st.Diff(g.device(args[0]).Label(), older, newer)
			if err != nil {
				return err
			}
			if d == "" {
				fmt.Println(theme.Dim.Render("no differences"))
				return nil
			}
			fmt.Println(render.Diff(d))
			return nil
		},
	}
	show := &cobra.Command{
		Use:   "show <device> [version]",
		Short: "Print a stored configuration (default: latest)",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			st, err := g.app.Backups()
			if err != nil {
				return err
			}
			label := g.device(args[0]).Label()
			es, err := st.List(label)
			if err != nil || len(es) == 0 {
				return fmt.Errorf("no backups for %s", label)
			}
			h := es[0].Hash
			if len(args) == 2 {
				h = ""
				for _, e := range es {
					if strings.HasPrefix(e.Hash, args[1]) {
						h = e.Hash
					}
				}
				if h == "" {
					return fmt.Errorf("version %s not found", args[1])
				}
			}
			c, err := st.Content(label, h)
			if err != nil {
				return err
			}
			fmt.Print(c)
			return nil
		},
	}
	c.AddCommand(list, diff, show)
	return c
}
