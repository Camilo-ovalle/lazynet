// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/render"
	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	nm := next.(model)
	nm.flushNotices()
	return nm, cmd
}

func (m model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		if m.busy > 0 {
			m.refreshContent(false)
		}
		return m, cmd

	case promptMsg:
		m.showPrompt(msg)
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.answerPrompt("", false)
			return m, tea.Quit
		}
		if m.modal != nil {
			return m.updateModal(msg)
		}
		return m.handleKey(msg)

	case tea.MouseMsg:
		if m.modal != nil {
			return m, nil
		}
		return m.handleMouse(msg)
	}
	return m.handleResult(msg)
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	d := m.current()
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "?":
		m.openModal(&modal{kind: modalHelp, title: "lazynet keys"})
	case "tab", "shift+tab":
		m.focus = 1 - m.focus
	case "h", "left":
		m.focus = focusDevices
	case "l", "right":
		m.focus = focusDetail
	case "1", "2", "3", "4":
		return m, m.selectTab(tab(msg.String()[0] - '1'))
	case "]":
		return m, m.selectTab((m.tab + 1) % numTabs)
	case "[":
		return m, m.selectTab((m.tab + numTabs - 1) % numTabs)
	case "up", "k":
		m.up()
	case "down", "j":
		m.down()
	case "pgup", "ctrl+u":
		m.vp.HalfPageUp()
	case "pgdown", "ctrl+d":
		m.vp.HalfPageDown()
	case "home", "g":
		m.vp.GotoTop()
	case "end", "G":
		m.vp.GotoBottom()
	case "enter":
		if d == nil {
			return m, nil
		}
		if m.tab == tabBackups && m.focus == focusDetail {
			return m, m.openDiff()
		}
		if m.tab == tabIface && m.state(d.Label()).ifacePort == "" {
			return m, m.openIfacePrompt()
		}
		m.focus = focusDetail
		return m, m.loadTab(true)
	case "r":
		return m, m.loadTab(true)
	case "p":
		return m, m.openIfacePrompt()
	case "w":
		m.showRaw = !m.showRaw
		m.refreshContent(false)
	case "b":
		if d != nil {
			if err := m.unsupported(d, platform.CapBackup); err != nil {
				m.setStatus(errText(err), true)
				return m, nil
			}
			m.tab = tabBackups
			m.setStatus("backing up "+d.Label()+"…", false)
			return m, m.start(d.Label(), tabBackups, takeBackup(m.app, m.env, *d))
		}
	case "v":
		return m, m.openVlanForm()
	case "a":
		m.openAdhoc()
	case "/":
		m.openGrep()
	case "f":
		m.openRange()
	case "s":
		switch m.logView.maxSev {
		case 7:
			m.logView.maxSev = 4
		case 4:
			m.logView.maxSev = 3
		default:
			m.logView.maxSev = 7
		}
		m.tab = tabLogs
		m.refreshContent(true)
		return m, m.loadTab(false)
	}
	return m, nil
}

func (m *model) up() {
	if m.focus == focusDevices {
		m.moveCursor(-1)
		return
	}
	if m.tab == tabBackups {
		if st := m.curState(); st != nil && st.backupSel > 0 {
			st.backupSel--
			m.refreshContent(false)
		}
		return
	}
	m.vp.ScrollUp(1)
}

func (m *model) down() {
	if m.focus == focusDevices {
		m.moveCursor(1)
		return
	}
	if m.tab == tabBackups {
		if st := m.curState(); st != nil && st.backupSel < len(st.backups)-1 {
			st.backupSel++
			m.refreshContent(false)
		}
		return
	}
	m.vp.ScrollDown(1)
}

func (m *model) curState() *devState {
	if d := m.current(); d != nil {
		return m.state(d.Label())
	}
	return nil
}

func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	lw := m.leftWidth()
	inLeft := msg.X < lw
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if inLeft {
			m.focus = focusDevices
		} else {
			m.focus = focusDetail
		}
		m.up()
		m.up()
		m.up()
		return m, nil
	case tea.MouseButtonWheelDown:
		if inLeft {
			m.focus = focusDevices
		} else {
			m.focus = focusDetail
		}
		m.down()
		m.down()
		m.down()
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if inLeft {
		m.focus = focusDevices
		i := m.listOffset + msg.Y - 1
		if msg.Y >= 1 && i >= 0 && i < len(m.rows) && m.rows[i].device != nil {
			m.cursor = i
			m.refreshContent(true)
			return m, m.loadTab(false)
		}
		return m, nil
	}
	m.focus = focusDetail
	if msg.Y == 0 {
		for t, r := range m.tabRanges() {
			if msg.X >= r[0] && msg.X < r[1] {
				t := tab(t)
				return m, m.selectTab(t)
			}
		}
	}
	return m, nil
}

// --- dialogs ---

func (m *model) openIfacePrompt() tea.Cmd {
	d := m.current()
	if d == nil {
		return nil
	}
	st := m.state(d.Label())
	if err := m.unsupported(d, platform.CapInterfaces); err != nil {
		st.iface, st.ifaceErr = nil, err
		m.tab = tabIface
		m.setStatus(errText(err), true)
		m.refreshContent(true)
		return nil
	}
	md := newInput("Query interface", "Interface on "+d.Label()+" (shorthand works: 1/0/25, gi1/0/25, eth1/1)",
		[3]string{"Interface", "Gi1/0/25", st.ifacePort})
	md.onSubmit = func(m *model, v []string) tea.Cmd {
		if v[0] == "" {
			return nil
		}
		st.ifacePort, st.iface, st.ifaceErr = v[0], nil, nil
		m.tab = tabIface
		m.focus = focusDetail
		m.setStatus("reading "+v[0]+" on "+d.Label()+"…", false)
		return m.start(d.Label(), tabIface, loadIface(m.app, m.env, *d, v[0]))
	}
	m.openModal(md)
	return nil
}

func (m *model) openGrep() {
	md := newInput("Filter logs", "Show only entries containing this text (empty clears)", [3]string{"Text", "LINK-3-UPDOWN", m.logView.grep})
	md.onSubmit = func(m *model, v []string) tea.Cmd {
		m.logView.grep = v[0]
		m.tab = tabLogs
		m.refreshContent(true)
		return m.loadTab(false)
	}
	m.openModal(md)
}

func (m *model) openRange() {
	md := newInput("Log time range", "Device local time. Formats: 2026-11-15 08:00 or 2026-11-15.\nLeave a field empty for no limit.",
		[3]string{"From", "2026-11-15 08:00", fmtTime(m.logView.from)},
		[3]string{"To", "2026-11-15 18:00", fmtTime(m.logView.to)})
	md.onSubmit = func(m *model, v []string) tea.Cmd {
		var lv = m.logView
		var err error
		lv.from, lv.to = parseTimeOrZero(v[0], &err, false), parseTimeOrZero(v[1], &err, true)
		if err != nil {
			m.setStatus("invalid time: use YYYY-MM-DD HH:MM", true)
			return nil
		}
		m.logView = lv
		m.tab = tabLogs
		m.refreshContent(true)
		return m.loadTab(false)
	}
	m.openModal(md)
}

func parseTimeOrZero(s string, errp *error, endOfDay bool) (t time.Time) {
	if s == "" {
		return
	}
	v, err := parse.ParseUserTime(s)
	if err != nil {
		*errp = err
		return
	}
	if endOfDay && len(s) == 10 {
		v = v.Add(24*time.Hour - time.Second)
	}
	return v
}

func (m *model) openAdhoc() {
	md := newInput("Connect to a device", "Not in the inventory? Connect by address.", [3]string{"Host", "192.168.100.100 or host:port", ""}, [3]string{"Username", "admin (optional)", ""})
	md.optional = []bool{false, true}
	md.onSubmit = func(m *model, v []string) tea.Cmd {
		if v[0] == "" {
			return nil
		}
		d := m.app.Inv.Resolve(v[0])
		if d.Name == "" {
			if v[1] != "" {
				d.Username = v[1]
			}
			d.Name, d.Group = d.Host, "ad-hoc"
			if d.SSHPort() != 22 {
				d.Name = d.Addr()
			}
			m.app.Inv.Devices = append(m.app.Inv.Devices, d)
		}
		m.buildRows()
		for i, r := range m.rows {
			if r.device != nil && r.device.Label() == d.Label() {
				m.cursor = i
			}
		}
		m.ensureVisible()
		m.tab = tabInfo
		m.refreshContent(true)
		return m.loadTab(true)
	}
	m.openModal(md)
}

func (m *model) openVlanForm() tea.Cmd {
	d := m.current()
	if d == nil {
		return nil
	}
	if err := m.unsupported(d, platform.CapVlanChange); err != nil {
		m.setStatus(d.Label()+": "+errText(err), true)
		return nil
	}
	port := m.state(d.Label()).ifacePort
	md := newInput("Change access VLAN · "+d.Label(),
		"lazynet will first check (read-only) that the port is an access port and\nthat the VLAN exists, then show you the exact commands before applying.",
		[3]string{"Port", "Gi1/0/1", port}, [3]string{"VLAN", "110", ""})
	md.onSubmit = func(m *model, v []string) tea.Cmd {
		vlan, err := strconv.Atoi(v[1])
		if err != nil {
			m.setStatus("VLAN must be a number", true)
			return nil
		}
		// The driver validates the port strictly before using it (the
		// injection barrier); a bad port is reported by the plan.
		port := strings.TrimSpace(v[0])
		if port == "" {
			m.setStatus("the port is required", true)
			return nil
		}
		m.pending = &pendingVlan{dev: *d, port: port, vlan: vlan}
		m.setStatus("checking "+port+" and VLAN "+v[1]+" on "+d.Label()+" (read-only)…", false)
		return m.start(d.Label(), tabIface, planVlan(m.app, m.env, *d, port, vlan))
	}
	m.openModal(md)
	return nil
}

func (m *model) openDiff() tea.Cmd {
	d := m.current()
	st := m.curState()
	if d == nil || st == nil || len(st.backups) == 0 {
		return nil
	}
	return loadDiff(m.app, *d, st.backups[st.backupSel].Hash)
}

// --- results from background operations ---

func (m model) handleResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case infoMsg:
		st := m.done(msg.label, tabInfo)
		st.info, st.infoErr = nil, msg.err
		if msg.err == nil {
			st.info = &msg.info
			m.platforms[msg.label] = msg.info.Platform
			m.setStatus("connected to "+msg.label, false)
		} else {
			m.setStatus(msg.label+": "+msg.err.Error(), true)
		}
	case ifaceMsg:
		st := m.done(msg.label, tabIface)
		st.iface, st.ifaceErr = nil, msg.err
		if msg.err == nil {
			st.iface = &msg.detail
			m.setStatus(msg.detail.Name+" on "+msg.label, false)
		} else {
			m.setStatus(msg.err.Error(), true)
		}
	case logsMsg:
		st := m.done(msg.label, tabLogs)
		st.logs, st.logsErr = nil, msg.err
		if msg.err == nil {
			st.logs = &msg.res
			m.setStatus(fmt.Sprintf("%s from %s", plural(len(msg.res.Entries), "log entry"), msg.label), false)
		} else {
			m.setStatus(msg.err.Error(), true)
		}
	case backupListMsg:
		st := m.state(msg.label)
		st.backups, st.backupsErr = msg.entries, msg.err
		st.backupSel = min(st.backupSel, max(0, len(st.backups)-1))
	case backupDoneMsg:
		st := m.done(msg.label, tabBackups)
		if msg.err != nil {
			m.setStatus("backup of "+msg.label+" failed: "+msg.err.Error(), true)
		} else {
			if msg.res.Changed {
				m.setStatus("✓ backup of "+msg.label+" saved ("+msg.res.Hash[:8]+")", false)
			} else {
				m.setStatus("✓ "+msg.label+" unchanged since last backup ("+msg.res.Hash[:8]+")", false)
			}
			st.backupSel, st.diff, st.diffHash, st.diffErr = 0, "", "", nil
		}
		if d := m.current(); d != nil && d.Label() == msg.label {
			m.refreshContent(false)
			return m, listBackups(m.app, *d)
		}
	case diffMsg:
		st := m.state(msg.label)
		st.diff, st.diffHash, st.diffErr = msg.text, msg.hash, msg.err
	case planMsg:
		m.done(msg.dev.Label(), tabIface)
		return m.onPlan(msg)
	case applyMsg:
		m.done(msg.dev.Label(), tabIface)
		return m.onApply(msg)
	case updateMsg:
		// A failed check stays silent: offline networks are normal here.
		if msg.err == nil && msg.avail.Version != "" {
			m.queueNotice(newUpdateBanner(msg.avail))
		}
		return m, nil
	case saveMsg:
		m.done(msg.dev.Label(), tabIface)
		if msg.err != nil {
			m.setStatus("save failed: "+msg.err.Error(), true)
		} else {
			m.setStatus("✓ configuration saved on "+msg.dev.Label()+" (audit: "+m.app.Audit.Path()+")", false)
		}
		m.pending = nil
	default:
		return m, nil
	}
	m.refreshContent(false)
	return m, nil
}

func (m model) onPlan(msg planMsg) (tea.Model, tea.Cmd) {
	p := m.pending
	if p == nil {
		return m, nil
	}
	if msg.err != nil {
		m.pending = nil
		m.setStatus("VLAN change refused: "+msg.err.Error(), true)
		m.refreshContent(false)
		return m, nil
	}
	m.platforms[msg.dev.Label()] = msg.platform
	if msg.plan.NoOp {
		m.pending = nil
		m.setStatus(fmt.Sprintf("%s is already in VLAN %d: nothing to do", msg.plan.Port, msg.plan.TargetVlan), false)
		return m, nil
	}
	p.plan, p.user, p.shown = msg.plan, msg.user, msg.plan.Change.Display()
	body := render.KV([][2]string{
		{"Device", msg.dev.Label() + " (" + msg.dev.Host + ")"},
		{"Port", msg.plan.Port},
		{"Mode", msg.plan.PortMode},
		{"Current", fmt.Sprintf("VLAN %d %s", msg.plan.CurrentVlan, theme.Dim.Render(msg.plan.CurrentVlanName))},
		{"Target", fmt.Sprintf("VLAN %d %s", msg.plan.TargetVlan, theme.Dim.Render(msg.plan.TargetVlanName))},
	}) + "\n\n" + theme.Bold.Render("These commands will be sent:") + "\n" + render.Commands(p.shown)
	if w := render.Warnings(msg.plan.Warnings); w != "" {
		body += "\n\n" + w
	}
	m.setStatus("review the change", false)
	m.openModal(newConfirm("Confirm configuration change", body, true, func(m *model, yes bool) tea.Cmd {
		if !yes {
			m.app.DeclineVlan(p.dev, p.user, p.plan)
			m.pending = nil
			m.setStatus("Aborted: nothing was changed", false)
			return nil
		}
		m.setStatus("applying on "+p.dev.Label()+"…", false)
		return m.start(p.dev.Label(), tabIface, applyVlan(m.app, m.env, p.dev, p.port, p.vlan, p.shown))
	}))
	return m, nil
}

func (m model) onApply(msg applyMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.pending = nil
		if errors.Is(msg.err, errStateChanged) {
			m.setStatus(msg.err.Error(), true)
		} else {
			m.setStatus("VLAN change failed: "+msg.err.Error(), true)
		}
		return m, nil
	}
	// Device info is unaffected by a VLAN change; only the port is. Re-read
	// that port so the Interface tab shows the new VLAN as the device reports it.
	st := m.state(msg.dev.Label())
	st.iface, st.ifaceErr, st.ifacePort = nil, nil, msg.plan.Port
	reload := m.start(msg.dev.Label(), tabIface, loadIface(m.app, m.env, msg.dev, msg.plan.Port))
	m.setStatus(fmt.Sprintf("✓ %s is now in VLAN %d (verified)", msg.plan.Port, msg.plan.TargetVlan), false)
	if msg.plan.Save == nil {
		m.pending = nil
		m.refreshContent(false)
		return m, reload
	}
	save := msg.plan.Save.Display()
	dev := msg.dev
	m.openModal(newConfirm("Save configuration?",
		theme.Good.Render(fmt.Sprintf("✓ %s is now in VLAN %d.", msg.plan.Port, msg.plan.TargetVlan))+"\n\n"+
			"Save running-config to startup-config? This will send:\n"+render.Commands(save)+"\n\n"+
			theme.Dim.Render("If you don't save, the change is lost on reload."),
		true, func(m *model, yes bool) tea.Cmd {
			if !yes {
				m.pending = nil
				m.setStatus("Not saved: change is in running-config only", false)
				return nil
			}
			m.setStatus("saving configuration…", false)
			return m.start(dev.Label(), tabIface, saveConfig(m.app, m.env, dev, save))
		}))
	m.refreshContent(false)
	return m, reload
}
