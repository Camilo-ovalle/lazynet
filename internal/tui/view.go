// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/Camilo-ovalle/lazynet/internal/render"
	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const minW, minH = 60, 12

func (m model) leftWidth() int { return max(26, min(38, m.width/4)) }

func (m model) mainHeight() int { return m.height - 1 }

func (m *model) resize() {
	m.vp.Width = max(10, m.width-m.leftWidth()-4)
	m.vp.Height = max(3, m.mainHeight()-2)
	m.ensureVisible()
	m.refreshContent(false)
}

// box draws a rounded panel of exactly w×h cells with a title in the border.
func box(title string, lines []string, w, h int, focused bool) []string {
	bc := lipgloss.NewStyle().Foreground(theme.Subtle)
	if focused {
		bc = lipgloss.NewStyle().Foreground(theme.Accent)
	}
	inner := w - 2
	tw := lipgloss.Width(title)
	if tw > inner-3 {
		title = ansi.Truncate(title, max(0, inner-3), "")
		tw = lipgloss.Width(title)
	}
	out := []string{bc.Render("╭─ ") + title + bc.Render(" "+strings.Repeat("─", max(0, inner-3-tw))+"╮")}
	for i := 0; i < h-2; i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		l = ansi.Truncate(l, inner-2, "…")
		out = append(out, bc.Render("│")+" "+l+strings.Repeat(" ", max(0, inner-2-lipgloss.Width(l)))+" "+bc.Render("│"))
	}
	out = append(out, bc.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return out
}

// ensureVisible scrolls the device list so the cursor (and, when possible,
// its group header) is on screen. Called from Update, never from View.
func (m *model) ensureVisible() {
	visible := m.mainHeight() - 2
	if visible < 1 || m.cursor < 0 {
		return
	}
	if m.cursor < m.listOffset {
		m.listOffset = m.cursor
	}
	if m.cursor >= m.listOffset+visible {
		m.listOffset = m.cursor - visible + 1
	}
	if m.listOffset > 0 && m.listOffset == m.cursor && m.rows[m.listOffset-1].device == nil {
		m.listOffset--
	}
}

func (m model) deviceLines(h int) []string {
	visible := h - 2
	inner := m.leftWidth() - 4
	var out []string
	for i := m.listOffset; i < len(m.rows) && len(out) < visible; i++ {
		r := m.rows[i]
		if r.device == nil {
			out = append(out, theme.Group.Render(strings.ToUpper(r.group)))
			continue
		}
		name := r.device.Label()
		badge := render.PlatformBadge(m.platformOf(*r.device))
		if st, ok := m.states[name]; ok && st.loading[m.tab] {
			badge = m.spin.View() + "  "
		}
		nameW := inner - 7
		name = ansi.Truncate(name, nameW, "…")
		pad := strings.Repeat(" ", max(1, nameW-lipgloss.Width(name)+1))
		if i == m.cursor {
			st := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
			if m.focus == focusDevices {
				st = st.Reverse(true)
			}
			out = append(out, st.Render("› "+name+pad)+" "+badge)
		} else {
			out = append(out, "  "+name+pad+" "+badge)
		}
	}
	return out
}

func tabLabel(t tab) string { return " " + string(rune('1'+t)) + " " + tabNames[t] + " " }

// tabRanges returns the [start,end) screen columns of each tab label in the
// right panel's top border (used for mouse hit-testing).
func (m model) tabRanges() (r [numTabs][2]int) {
	x := m.leftWidth() + 3 // after "╭─ "
	for t := tab(0); t < numTabs; t++ {
		w := lipgloss.Width(tabLabel(t))
		r[t] = [2]int{x, x + w}
		x += w + 1 // separator
	}
	return r
}

func (m model) tabsTitle() string {
	var parts []string
	for t := tab(0); t < numTabs; t++ {
		label := tabLabel(t)
		if t == m.tab {
			parts = append(parts, lipgloss.NewStyle().Bold(true).Foreground(theme.Accent).Reverse(m.focus == focusDetail).Render(label))
		} else {
			parts = append(parts, theme.Dim.Render(label))
		}
	}
	return strings.Join(parts, theme.Dim.Render("│"))
}

func (m model) View() string {
	if m.width < minW || m.height < minH {
		return "lazynet needs at least 60×12 — resize your terminal (q to quit)"
	}
	h := m.mainHeight()
	lw := m.leftWidth()
	title := "Devices"
	if m.app.Demo {
		title += theme.Caution.Render(" demo")
	}
	left := box(theme.Bold.Render(title), m.deviceLines(h), lw, h, m.focus == focusDevices)
	right := box(m.tabsTitle(), strings.Split(m.vp.View(), "\n"), m.width-lw, h, m.focus == focusDetail)
	lines := make([]string, 0, m.height)
	for i := 0; i < h; i++ {
		lines = append(lines, left[i]+right[i])
	}
	lines = append(lines, m.statusLine())
	if m.modal != nil {
		lines = overlay(lines, m.modal.view(m.width), m.width)
	}
	return strings.Join(lines, "\n")
}

// statusLine is a full-width bar: a state icon and the last message on the
// left, global keys on the right. Every segment carries the bar background,
// because a nested style's reset would otherwise punch holes in it.
func (m model) statusLine() string {
	bg := theme.BarBg
	on := func(st lipgloss.Style, s string) string { return st.Background(bg).Render(s) }
	icon, msgStyle := on(theme.Key, "●"), lipgloss.NewStyle().Foreground(theme.Text)
	switch {
	case m.busy > 0:
		icon = on(lipgloss.NewStyle().Foreground(theme.Warn), ansi.Strip(m.spin.View()))
	case m.isError:
		icon, msgStyle = on(lipgloss.NewStyle().Foreground(theme.Danger).Bold(true), "✗"), lipgloss.NewStyle().Foreground(theme.Danger)
	case strings.HasPrefix(m.status, "✓"):
		icon = on(lipgloss.NewStyle().Foreground(theme.Success).Bold(true), "✓")
	}
	text := strings.TrimSpace(strings.TrimPrefix(m.status, "✓"))
	keys := func(pairs ...string) string {
		var b strings.Builder
		for i := 0; i < len(pairs); i += 2 {
			if i > 0 {
				b.WriteString(on(theme.Dim, "  "))
			}
			b.WriteString(on(theme.Key, pairs[i]) + on(theme.Dim, " "+pairs[i+1]))
		}
		return b.String()
	}
	right := keys("?", "help", "q", "quit") + on(lipgloss.NewStyle(), " ")
	left := on(lipgloss.NewStyle(), " ") + icon + on(lipgloss.NewStyle(), " ")
	room := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 3 // keep a gap before the keys
	msg := on(msgStyle, ansi.Truncate(text, max(0, room), "…"))
	gap := on(lipgloss.NewStyle(), strings.Repeat(" ", max(1, m.width-lipgloss.Width(left)-lipgloss.Width(msg)-lipgloss.Width(right))))
	return left + msg + gap + right
}

// overlay draws fg centered on top of the base lines.
func overlay(base []string, fg string, width int) []string {
	fl := strings.Split(fg, "\n")
	fw := lipgloss.Width(fg)
	x := max(0, (width-fw)/2)
	y := max(0, (len(base)-len(fl))/2)
	for i, l := range fl {
		if y+i >= len(base) {
			break
		}
		b := base[y+i]
		leftPart := ansi.Truncate(b, x, "")
		leftPart += strings.Repeat(" ", max(0, x-lipgloss.Width(leftPart)))
		rightPart := ansi.TruncateLeft(b, x+fw, "")
		base[y+i] = leftPart + l + rightPart
	}
	return base
}
