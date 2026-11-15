// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package theme holds the colors and styles shared by the CLI and the TUI.
package theme

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	Accent  = lipgloss.AdaptiveColor{Light: "#0B7A75", Dark: "#5FD7AF"}
	Accent2 = lipgloss.AdaptiveColor{Light: "#1F5FAD", Dark: "#87AFFF"}
	Muted   = lipgloss.AdaptiveColor{Light: "#6C6C6C", Dark: "#8A8A8A"}
	Subtle  = lipgloss.AdaptiveColor{Light: "#C6C6C6", Dark: "#3A3A3A"}
	Text    = lipgloss.AdaptiveColor{Light: "#1C1C1C", Dark: "#E4E4E4"}
	Warn    = lipgloss.AdaptiveColor{Light: "#A15C00", Dark: "#FFD75F"}
	Danger  = lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#FF6B6B"}
	Success = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#87D787"}

	Title   = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	Label   = lipgloss.NewStyle().Foreground(Muted)
	Value   = lipgloss.NewStyle().Foreground(Text)
	Dim     = lipgloss.NewStyle().Foreground(Muted)
	Bold    = lipgloss.NewStyle().Bold(true)
	Good    = lipgloss.NewStyle().Foreground(Success)
	Bad     = lipgloss.NewStyle().Foreground(Danger)
	Caution = lipgloss.NewStyle().Foreground(Warn)
	Command = lipgloss.NewStyle().Foreground(Warn).Bold(true)
	Header  = lipgloss.NewStyle().Bold(true).Foreground(Accent2)
)

// PlatformBadge renders a short colored platform tag.
func PlatformBadge(p string) string {
	c, txt := Muted, "  ?  "
	switch p {
	case "catalyst":
		c, txt = lipgloss.AdaptiveColor{Light: "#1F5FAD", Dark: "#5FAFFF"}, "CAT"
	case "nexus":
		c, txt = lipgloss.AdaptiveColor{Light: "#6A1B9A", Dark: "#D787FF"}, "NXS"
	case "asa":
		c, txt = lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#FF8787"}, "ASA"
	default:
		txt = " ? "
	}
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render(txt)
}

// Severity styles a syslog severity (0-7) for display.
func Severity(sev int) (string, lipgloss.Style) {
	switch {
	case sev < 0:
		return "---", Dim
	case sev <= 2:
		return [...]string{"EMR", "ALR", "CRT"}[sev], lipgloss.NewStyle().Foreground(Danger).Bold(true)
	case sev == 3:
		return "ERR", lipgloss.NewStyle().Foreground(Danger)
	case sev == 4:
		return "WRN", lipgloss.NewStyle().Foreground(Warn)
	case sev == 5:
		return "NTC", lipgloss.NewStyle().Foreground(Accent2)
	case sev == 6:
		return "INF", lipgloss.NewStyle().Foreground(Text)
	}
	return "DBG", Dim
}
