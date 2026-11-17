// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package render formats device data for the terminal. It is shared by the
// CLI and the TUI so both look the same.
package render

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// PlatformBadge renders the registered badge of a platform ("CAT"), or a dim
// placeholder for an unknown one.
func PlatformBadge(id platform.ID) string {
	d, ok := platform.Lookup(id)
	if !ok {
		return theme.Badge("", theme.Subtle)
	}
	return theme.Badge(d.Badge.Text, lipgloss.AdaptiveColor{Light: d.Badge.Light, Dark: d.Badge.Dark})
}

// KV renders aligned label/value rows.
func KV(rows [][2]string) string {
	w := 0
	for _, r := range rows {
		w = max(w, len(r[0]))
	}
	var b strings.Builder
	for _, r := range rows {
		v := r[1]
		if v == "" {
			v = theme.Dim.Render("—")
		}
		b.WriteString("  " + theme.Label.Render(fmt.Sprintf("%-*s", w, r[0])) + "  " + theme.Value.Render(v) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// Info renders basic device information.
func Info(label string, i platform.Info) string {
	clk := ""
	if !i.Clock.IsZero() {
		clk = i.Clock.Format("2006-01-02 15:04:05") + theme.Dim.Render(" (device local)")
	}
	head := theme.Title.Render(label) + "  " + PlatformBadge(i.Platform) + " " + theme.Dim.Render(i.OS+" "+i.Version)
	return head + "\n" + KV([][2]string{
		{"Hostname", i.Hostname}, {"Model", i.Model}, {"Serial", i.Serial}, {"MAC", i.MAC},
		{"IP", i.IP}, {"OS version", strings.TrimSpace(i.OS + " " + i.Version)}, {"Uptime", i.Uptime}, {"Clock", clk},
	})
}

// Status colors an interface status.
func Status(s string) string {
	switch s {
	case "up":
		return theme.Good.Render(s)
	case "":
		return ""
	}
	return theme.Bad.Render(s)
}

// Interface renders one interface.
func Interface(d platform.InterfaceDetail) string {
	status := Status(d.Status)
	if d.Reason != "" {
		status += theme.Dim.Render(" (" + d.Reason + ")")
	}
	vlan := ""
	if d.Switchport != nil && d.Mode == "access" {
		vlan = strconv.Itoa(d.AccessVlan)
		if d.Switchport.VlanName != "" {
			vlan += theme.Dim.Render(" " + d.Switchport.VlanName)
		}
	}
	errs := fmt.Sprintf("in %d · crc %d · out %d", d.InputErrors, d.CRC, d.OutputErrors)
	if d.InputErrors+d.CRC+d.OutputErrors > 0 {
		errs = theme.Caution.Render(errs)
	}
	speed := d.Speed
	if d.Duplex != "" {
		speed += theme.Dim.Render(" " + d.Duplex + "-duplex")
	}
	return theme.Title.Render(d.Name) + "\n" + KV([][2]string{
		{"Status", status}, {"Protocol", d.Protocol}, {"Description", d.Description}, {"Mode", d.Mode},
		{"Access VLAN", vlan}, {"Speed", speed}, {"MAC", d.MAC}, {"Errors", errs},
		{"Input rate", d.InputRate}, {"Output rate", d.OutputRate},
	})
}

// Log renders one log entry on a single line.
func Log(e parse.LogEntry) string {
	ts := "                   "
	if e.HasTime {
		ts = e.Time.Format("2006-01-02 15:04:05")
	}
	tag, st := theme.Severity(e.Severity)
	mn := e.Facility
	if e.Mnemonic != "" {
		mn = fmt.Sprintf("%s-%d-%s", e.Facility, e.Severity, e.Mnemonic)
	}
	return theme.Dim.Render(ts) + " " + st.Render(tag) + " " + theme.Header.Render(mn) + " " + e.Message
}

// Diff colors a unified diff.
func Diff(d string) string {
	lines := strings.Split(strings.TrimRight(d, "\n"), "\n")
	for i, l := range lines {
		switch {
		case strings.HasPrefix(l, "+++") || strings.HasPrefix(l, "---") || strings.HasPrefix(l, "diff "):
			lines[i] = theme.Bold.Render(l)
		case strings.HasPrefix(l, "+"):
			lines[i] = theme.Good.Render(l)
		case strings.HasPrefix(l, "-"):
			lines[i] = theme.Bad.Render(l)
		case strings.HasPrefix(l, "@@"):
			lines[i] = theme.Header.Render(l)
		}
	}
	return strings.Join(lines, "\n")
}

// Commands renders device commands that are about to be sent.
func Commands(lines []string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString("  " + theme.Command.Render(l) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// ReleaseNotes renders the markdown of a changelog section for the terminal,
// wrapped to width: "### Added" becomes a heading, "- item" a bullet, and
// `code` is highlighted. Source lines wrapped by hand are joined first.
func ReleaseNotes(md string, width int) []string {
	width = max(width, 20)
	type block struct {
		kind string // "h", "li", "p" or "" (blank line)
		text string
	}
	var blocks []block
	for _, l := range strings.Split(md, "\n") {
		t := strings.TrimSpace(l)
		switch {
		case t == "":
			if len(blocks) > 0 && blocks[len(blocks)-1].kind != "" {
				blocks = append(blocks, block{})
			}
		case strings.HasPrefix(t, "#"):
			blocks = append(blocks, block{"h", strings.TrimSpace(strings.TrimLeft(t, "#"))})
		case strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* "):
			blocks = append(blocks, block{"li", t[2:]})
		case len(blocks) > 0 && (blocks[len(blocks)-1].kind == "li" || blocks[len(blocks)-1].kind == "p"):
			blocks[len(blocks)-1].text += " " + t
		default:
			blocks = append(blocks, block{"p", t})
		}
	}
	var out []string
	for _, b := range blocks {
		switch b.kind {
		case "":
			out = append(out, "")
		case "h":
			out = append(out, theme.Header.Render(b.text))
		case "li":
			for i, l := range strings.Split(ansi.Wordwrap(inlineCode(b.text), width-4, ""), "\n") {
				prefix := "    "
				if i == 0 {
					prefix = "  " + theme.Title.Render("•") + " "
				}
				out = append(out, prefix+l)
			}
		default:
			out = append(out, strings.Split(ansi.Wordwrap(inlineCode(b.text), width, ""), "\n")...)
		}
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// inlineCode highlights `code` spans.
func inlineCode(s string) string {
	parts := strings.Split(s, "`")
	if len(parts)%2 == 0 { // unbalanced: leave it alone
		return s
	}
	for i := 1; i < len(parts); i += 2 {
		parts[i] = theme.Key.Render(parts[i])
	}
	return strings.Join(parts, "")
}
