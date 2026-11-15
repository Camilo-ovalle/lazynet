// Package tui is the interactive Bubble Tea interface: a device list on the
// left, tabs with details on the right, modal dialogs for input and for
// confirming changes. Mouse is supported (click to select, wheel to scroll).
package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Camilo-ovalle/Lazy_Net/internal/app"
	"github.com/Camilo-ovalle/Lazy_Net/internal/backup"
	"github.com/Camilo-ovalle/Lazy_Net/internal/device"
	"github.com/Camilo-ovalle/Lazy_Net/internal/inventory"
	"github.com/Camilo-ovalle/Lazy_Net/internal/parse"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type tab int

const (
	tabInfo tab = iota
	tabIface
	tabLogs
	tabBackups
	numTabs
)

var tabNames = [numTabs]string{"Info", "Interface", "Logs", "Backups"}

type focus int

const (
	focusDevices focus = iota
	focusDetail
)

// row is one line of the device list: a group header or a device.
type row struct {
	group  string
	device *inventory.Device
}

// devState caches everything loaded for one device.
type devState struct {
	info       *device.Info
	infoErr    error
	iface      *device.InterfaceDetail
	ifaceErr   error
	ifacePort  string
	logs       *device.LogResult
	logsErr    error
	backups    []backup.Entry
	backupsErr error
	backupSel  int
	diff       string
	diffHash   string
	diffErr    error
	loading    map[tab]bool
}

type logView struct {
	from, to time.Time
	grep     string
	maxSev   int
}

type model struct {
	app *app.App
	env *env

	width, height int
	rows          []row
	cursor        int // index into rows (always a device row)
	listOffset    int
	focus         focus
	tab           tab
	states        map[string]*devState
	platforms     map[string]device.Platform
	logView       logView
	showRaw       bool

	vp      viewport.Model
	spin    spinner.Model
	busy    int
	status  string
	isError bool

	modal   *modal
	prompts []promptMsg // background prompts waiting for the current modal
	pending *pendingVlan
}

// pendingVlan tracks the VLAN change workflow between modals.
type pendingVlan struct {
	dev   inventory.Device
	user  string
	port  string
	vlan  int
	plan  *device.VlanPlan
	shown []string
}

// Run starts the TUI and blocks until the user quits.
func Run(a *app.App) error {
	e := &env{}
	m := newModel(a, e)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	e.program = p
	_, err := p.Run()
	return err
}

func newModel(a *app.App, e *env) model {
	sp := spinner.New(spinner.WithSpinner(spinner.MiniDot))
	m := model{
		app: a, env: e, states: map[string]*devState{}, platforms: a.DetectedPlatforms(),
		spin: sp, vp: viewport.New(0, 0), logView: logView{maxSev: 7},
	}
	m.buildRows()
	if len(a.Inv.Devices) == 0 {
		m.setStatus("Inventory is empty: press 'a' to connect to an IP, or create "+a.Paths.Inventory, false)
	} else {
		m.setStatus("Select a device and press Enter to connect · ? for help", false)
	}
	return m
}

func (m *model) buildRows() {
	var sel string
	if d := m.current(); d != nil {
		sel = d.Label()
	}
	m.rows = nil
	inv := m.app.Inv
	for _, g := range inv.Groups() {
		name := g
		if name == "" {
			name = "ungrouped"
		}
		m.rows = append(m.rows, row{group: name})
		var ds []*inventory.Device
		for i := range inv.Devices {
			if inv.Devices[i].Group == g {
				ds = append(ds, &inv.Devices[i])
			}
		}
		sort.SliceStable(ds, func(i, j int) bool { return ds[i].Label() < ds[j].Label() })
		for _, d := range ds {
			m.rows = append(m.rows, row{device: d})
		}
	}
	m.cursor = -1
	for i, r := range m.rows {
		if r.device != nil && (m.cursor < 0 || r.device.Label() == sel) {
			m.cursor = i
			if r.device.Label() == sel {
				break
			}
		}
	}
}

func (m model) current() *inventory.Device {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return m.rows[m.cursor].device
}

func (m *model) state(label string) *devState {
	s, ok := m.states[label]
	if !ok {
		s = &devState{loading: map[tab]bool{}}
		m.states[label] = s
	}
	return s
}

func (m *model) setStatus(s string, isErr bool) { m.status, m.isError = s, isErr }

func (m *model) moveCursor(delta int) {
	i := m.cursor
	for {
		i += delta
		if i < 0 || i >= len(m.rows) {
			return
		}
		if m.rows[i].device != nil {
			m.cursor = i
			m.ensureVisible()
			m.refreshContent(true)
			return
		}
	}
}

func (m model) Init() tea.Cmd { return m.spin.Tick }

// start marks a tab as loading and returns the command with busy accounting.
func (m *model) start(label string, t tab, cmd tea.Cmd) tea.Cmd {
	st := m.state(label)
	if st.loading[t] {
		return nil
	}
	st.loading[t] = true
	m.busy++
	m.refreshContent(false)
	return cmd
}

func (m *model) done(label string, t tab) *devState {
	st := m.state(label)
	if st.loading[t] {
		st.loading[t] = false
		m.busy--
	}
	return st
}

// loadTab fetches what the current tab needs, if not cached (or force).
func (m *model) loadTab(force bool) tea.Cmd {
	d := m.current()
	if d == nil {
		return nil
	}
	st := m.state(d.Label())
	switch m.tab {
	case tabInfo:
		if force || (st.info == nil && st.infoErr == nil) {
			m.setStatus("connecting to "+d.Label()+"…", false)
			return m.start(d.Label(), tabInfo, loadInfo(m.app, m.env, *d))
		}
	case tabLogs:
		if force || (st.logs == nil && st.logsErr == nil) {
			m.setStatus("reading log buffer of "+d.Label()+"…", false)
			return m.start(d.Label(), tabLogs, loadLogs(m.app, m.env, *d))
		}
	case tabBackups:
		return listBackups(m.app, *d)
	case tabIface:
		if force && st.ifacePort != "" {
			return m.start(d.Label(), tabIface, loadIface(m.app, m.env, *d, st.ifacePort))
		}
	}
	return nil
}

func (m *model) selectTab(t tab) tea.Cmd {
	m.tab = t
	m.vp.GotoTop()
	m.refreshContent(true)
	if m.tab == tabIface {
		if d := m.current(); d != nil && m.state(d.Label()).ifacePort == "" {
			return m.openIfacePrompt()
		}
	}
	return m.loadTab(false)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	return strings.ToUpper(s[:1]) + s[1:]
}

func (m *model) applyLogFilter(st *devState) ([]parse.LogEntry, int) {
	if st.logs == nil {
		return nil, 0
	}
	return parse.FilterLogs(st.logs.Entries, parse.LogFilter{
		From: m.logView.from, To: m.logView.to, Grep: m.logView.grep, MaxSeverity: m.logView.maxSev,
	})
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04")
}

func plural(n int, s string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, s)
	}
	if strings.HasSuffix(s, "y") {
		return fmt.Sprintf("%d %sies", n, strings.TrimSuffix(s, "y"))
	}
	return fmt.Sprintf("%d %ss", n, s)
}
