// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"

	"github.com/Camilo-ovalle/lazynet/internal/app"
	"github.com/Camilo-ovalle/lazynet/internal/backup"
	"github.com/Camilo-ovalle/lazynet/internal/inventory"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	tea "github.com/charmbracelet/bubbletea"
)

// Messages produced by background operations.
type (
	infoMsg struct {
		label string
		info  platform.Info
		err   error
	}
	ifaceMsg struct {
		label  string
		detail platform.InterfaceDetail
		err    error
	}
	logsMsg struct {
		label string
		res   platform.LogResult
		err   error
	}
	backupListMsg struct {
		label   string
		entries []backup.Entry
		err     error
	}
	backupDoneMsg struct {
		label string
		res   backup.Result
		err   error
	}
	diffMsg struct {
		label, hash, text string
		err               error
	}
	planMsg struct {
		dev      inventory.Device
		user     string
		platform platform.ID
		plan     *platform.VlanPlan
		err      error
	}
	applyMsg struct {
		dev  inventory.Device
		plan *platform.VlanPlan
		err  error
	}
	saveMsg struct {
		dev inventory.Device
		err error
	}
)

// withSession connects, runs fn and closes the session.
func withSession(a *app.App, e *env, d inventory.Device, fn func(*app.Session) error) error {
	s, err := a.Connect(d, prompter{e})
	if err != nil {
		return err
	}
	defer s.Close()
	return fn(s)
}

func loadInfo(a *app.App, e *env, d inventory.Device) tea.Cmd {
	return func() tea.Msg {
		var inf platform.Info
		err := withSession(a, e, d, func(s *app.Session) (err error) {
			inf, err = s.Info()
			return
		})
		return infoMsg{d.Label(), inf, err}
	}
}

func loadIface(a *app.App, e *env, d inventory.Device, port string) tea.Cmd {
	return func() tea.Msg {
		var det platform.InterfaceDetail
		err := withSession(a, e, d, func(s *app.Session) (err error) {
			det, err = s.Interface(port)
			return
		})
		return ifaceMsg{d.Label(), det, err}
	}
}

func loadLogs(a *app.App, e *env, d inventory.Device) tea.Cmd {
	return func() tea.Msg {
		var r platform.LogResult
		err := withSession(a, e, d, func(s *app.Session) (err error) {
			r, err = s.Logs()
			return
		})
		return logsMsg{d.Label(), r, err}
	}
}

func listBackups(a *app.App, d inventory.Device) tea.Cmd {
	return func() tea.Msg {
		st, err := a.Backups()
		if err != nil {
			return backupListMsg{label: d.Label(), err: err}
		}
		es, err := st.List(d.Label())
		return backupListMsg{d.Label(), es, err}
	}
}

func takeBackup(a *app.App, e *env, d inventory.Device) tea.Cmd {
	return func() tea.Msg {
		var r backup.Result
		err := withSession(a, e, d, func(s *app.Session) (err error) {
			r, err = a.Backup(s)
			return
		})
		return backupDoneMsg{d.Label(), r, err}
	}
}

func loadDiff(a *app.App, d inventory.Device, hash string) tea.Cmd {
	return func() tea.Msg {
		st, err := a.Backups()
		if err != nil {
			return diffMsg{label: d.Label(), err: err}
		}
		txt, err := st.Diff(d.Label(), "", hash)
		if err != nil {
			// First backup: show its full content instead of a diff.
			if c, cerr := st.Content(d.Label(), hash); cerr == nil {
				return diffMsg{d.Label(), hash, c, nil}
			}
		}
		return diffMsg{d.Label(), hash, txt, err}
	}
}

func planVlan(a *app.App, e *env, d inventory.Device, port string, vlan int) tea.Cmd {
	return func() tea.Msg {
		var p *platform.VlanPlan
		var user string
		var id platform.ID
		err := withSession(a, e, d, func(s *app.Session) (err error) {
			user, id = s.User, s.Platform
			p, err = a.PlanVlan(s, port, vlan)
			return
		})
		return planMsg{d, user, id, p, err}
	}
}

// errStateChanged means the device changed between review and apply.
var errStateChanged = errors.New("the device state changed since you reviewed the change: nothing was sent, please review it again")

// applyVlan re-validates on a fresh session and only applies if the commands
// are exactly the ones the user approved.
func applyVlan(a *app.App, e *env, d inventory.Device, port string, vlan int, shown []string) tea.Cmd {
	return func() tea.Msg {
		var plan *platform.VlanPlan
		err := withSession(a, e, d, func(s *app.Session) error {
			reviewed, err := a.PlanVlan(s, port, vlan)
			if err != nil {
				return err
			}
			plan = reviewed
			if plan.NoOp || plan.Change == nil || !plan.Change.Confirm(shown) {
				return errStateChanged
			}
			return a.ApplyVlan(s, plan)
		})
		return applyMsg{d, plan, err}
	}
}

func saveConfig(a *app.App, e *env, d inventory.Device, shown []string) tea.Cmd {
	return func() tea.Msg {
		err := withSession(a, e, d, func(s *app.Session) error {
			cr, err := a.SaveRequest(s)
			if err != nil {
				return err
			}
			if !cr.Confirm(shown) {
				return errStateChanged
			}
			return a.SaveConfig(s, cr)
		})
		return saveMsg{d, err}
	}
}
