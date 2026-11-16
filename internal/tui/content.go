// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"github.com/Camilo-ovalle/lazynet/internal/device"
	"strings"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/render"
	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/charmbracelet/x/ansi"
)

// refreshContent re-renders the detail pane into the viewport.
func (m *model) refreshContent(resetScroll bool) {
	m.vp.SetContent(m.content())
	if resetScroll {
		m.vp.GotoTop()
	}
}

func (m *model) loadingLine(what string) string {
	return m.spin.View() + " " + theme.Dim.Render(what)
}

func hint(keys ...string) string {
	var parts []string
	for i := 0; i+1 < len(keys); i += 2 {
		parts = append(parts, theme.Title.Render(keys[i])+" "+theme.Dim.Render(keys[i+1]))
	}
	return strings.Join(parts, theme.Dim.Render(" · "))
}

func (m *model) content() string {
	d := m.current()
	if d == nil {
		return theme.Dim.Render("No device selected.\n\nAdd devices to "+m.app.Paths.Inventory+"\nor press ") + theme.Title.Render("a") + theme.Dim.Render(" to connect to an IP address.")
	}
	st := m.state(d.Label())
	w := max(10, m.vp.Width)
	switch m.tab {
	case tabInfo:
		head := ""
		if d.Description != "" {
			head = theme.Dim.Render(d.Description) + "\n\n"
		}
		switch {
		case st.loading[tabInfo]:
			return head + m.loadingLine("connecting to "+d.Addr()+"…")
		case st.infoErr != nil:
			return head + theme.Bad.Render(errText(st.infoErr)) + "\n\n" + hint("enter", "retry")
		case st.info != nil:
			keys := []string{"r", "refresh", "p", "interface", "v", "VLAN change", "b", "backup"}
			if st.info.Platform == device.PaloAlto { // info and backups only, for now
				keys = []string{"r", "refresh", "b", "backup", "4", "backups"}
			}
			return head + render.Info(d.Label(), *st.info) + "\n\n" + hint(keys...)
		}
		return head + theme.Title.Render(d.Label()) + theme.Dim.Render("  "+d.Addr()) + "\n\n" +
			hint("enter", "connect and load device information")
	case tabIface:
		switch {
		case st.loading[tabIface]:
			return m.loadingLine("reading " + st.ifacePort + "…")
		case st.ifaceErr != nil:
			return theme.Bad.Render(errText(st.ifaceErr)) + "\n\n" + hint("p", "query another interface")
		case st.iface != nil:
			out := render.Interface(*st.iface) + "\n\n" + hint("p", "other interface", "r", "refresh", "w", "raw output", "v", "VLAN change")
			if m.showRaw {
				out += "\n\n" + theme.Dim.Render(st.iface.Raw)
			}
			return out
		}
		return hint("p", "query an interface (e.g. Gi1/0/25, 1/0/25, eth1/1)")
	case tabLogs:
		return m.logsContent(st, w)
	case tabBackups:
		return m.backupsContent(st, d.Label())
	}
	return ""
}

func (m *model) logsContent(st *devState, w int) string {
	switch {
	case st.loading[tabLogs]:
		return m.loadingLine("reading log buffer…")
	case st.logsErr != nil:
		return theme.Bad.Render(errText(st.logsErr)) + "\n\n" + hint("r", "retry")
	case st.logs == nil:
		return hint("enter", "load the log buffer")
	}
	kept, undated := m.applyLogFilter(st)
	rng := "all"
	if !m.logView.from.IsZero() || !m.logView.to.IsZero() {
		rng = fmtTime(m.logView.from) + " → " + fmtTime(m.logView.to)
	}
	sev := "all"
	if m.logView.maxSev < 7 {
		sev = parse.SeverityName[m.logView.maxSev] + " and worse"
	}
	grep := m.logView.grep
	if grep == "" {
		grep = "—"
	}
	head := render.KV([][2]string{{"Range", rng}, {"Severity", sev}, {"Filter", grep},
		{"Showing", fmt.Sprintf("%d of %s", len(kept), plural(len(st.logs.Entries), "entry"))}})
	if undated > 0 {
		head += "\n" + theme.Caution.Render(fmt.Sprintf("  %d entries have no date (uptime timestamps) and are hidden by the range filter", undated))
	}
	var b strings.Builder
	b.WriteString(head + "\n" + hint("f", "range", "/", "filter", "s", "severity", "r", "reload") + "\n\n")
	if len(kept) == 0 {
		b.WriteString(theme.Dim.Render("no entries match"))
	}
	for i := len(kept) - 1; i >= 0; i-- { // newest first
		b.WriteString(ansi.Truncate(render.Log(kept[i]), w, "…") + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *model) backupsContent(st *devState, label string) string {
	var b strings.Builder
	if st.loading[tabBackups] {
		b.WriteString(m.loadingLine("taking backup…") + "\n\n")
	}
	if st.backupsErr != nil {
		return theme.Bad.Render(errText(st.backupsErr))
	}
	if len(st.backups) == 0 {
		b.WriteString(theme.Dim.Render("No backups of "+label+" yet.") + "\n\n" + hint("b", "take a backup now"))
		return b.String()
	}
	for i, e := range st.backups {
		line := fmt.Sprintf("%s  %s", e.Hash[:8], e.Time.Local().Format("2006-01-02 15:04:05"))
		if i == 0 {
			line += theme.Dim.Render("  latest")
		}
		if i == st.backupSel {
			b.WriteString(theme.Title.Render("› ") + theme.Bold.Render(line) + "\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}
	b.WriteString("\n" + hint("enter", "diff vs previous", "b", "backup now") + "\n")
	switch {
	case st.diffErr != nil:
		b.WriteString("\n" + theme.Caution.Render(errText(st.diffErr)))
	case st.diffHash != "" && st.diff == "":
		b.WriteString("\n" + theme.Dim.Render("no differences"))
	case st.diff != "":
		b.WriteString("\n" + render.Diff(st.diff))
	}
	return strings.TrimRight(b.String(), "\n")
}
