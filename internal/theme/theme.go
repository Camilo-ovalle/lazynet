// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package theme holds the colors and styles shared by the CLI and the TUI.
package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Colors follow the lazynet brand palette (assets/assets-generator) on dark
// terminals: mint for focus and actions, amber for commands and warnings.
// Light variants keep at least ~4.5:1 contrast on white backgrounds.
var (
	Accent  = lipgloss.AdaptiveColor{Light: "#0B7A57", Dark: "#3FE0A0"}
	Accent2 = lipgloss.AdaptiveColor{Light: "#1F5FAD", Dark: "#87AFFF"}
	Muted   = lipgloss.AdaptiveColor{Light: "#5F6B7A", Dark: "#8A97A8"}
	Subtle  = lipgloss.AdaptiveColor{Light: "#B8BEC6", Dark: "#4B5563"}
	Text    = lipgloss.AdaptiveColor{Light: "#1C1C1C", Dark: "#E8ECEF"}
	Warn    = lipgloss.AdaptiveColor{Light: "#9A5B00", Dark: "#FFB547"}
	Danger  = lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#FF6B6B"}
	Success = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#87D787"}

	// Surfaces used by the TUI.
	BarBg     = lipgloss.AdaptiveColor{Light: "#E6EAEE", Dark: "#1A222D"}
	Selection = lipgloss.AdaptiveColor{Light: "#CFE8DD", Dark: "#2A3A40"}

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
	Key     = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	Group   = lipgloss.NewStyle().Bold(true).Foreground(Muted)
)

// PlatformBadge renders a short colored platform tag.
func PlatformBadge(p string) string {
	c, txt := Subtle, " — "
	switch p {
	case "catalyst":
		c, txt = lipgloss.AdaptiveColor{Light: "#1F5FAD", Dark: "#5FAFFF"}, "CAT"
	case "nexus":
		c, txt = lipgloss.AdaptiveColor{Light: "#6A1B9A", Dark: "#D787FF"}, "NXS"
	case "asa":
		c, txt = lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#FF8787"}, "ASA"
	case "paloalto":
		c, txt = lipgloss.AdaptiveColor{Light: "#B4430B", Dark: "#FF9E64"}, "PAN"
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
