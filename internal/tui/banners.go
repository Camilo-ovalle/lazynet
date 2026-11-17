// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"github.com/Camilo-ovalle/lazynet/internal/render"
	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/Camilo-ovalle/lazynet/internal/update"
	tea "github.com/charmbracelet/bubbletea"
)

// newUpdateBanner offers a newer release: update now, skip that version, or
// decide later. Updating closes the UI; the caller installs the release in
// the terminal (sudo may ask for a password) and starts the new binary.
func newUpdateBanner(av update.Available) *modal {
	body := theme.Bold.Render("lazynet "+av.Version) + " is available. You have " + av.Current + ".\n\n" +
		"Updating closes lazynet, installs the new version the same way\n" +
		"this one was installed, and opens lazynet again.\n\n" +
		theme.Dim.Render("Release notes:") + "\n" + theme.Dim.Render(av.URL)
	return &modal{
		kind: modalChoice, title: "Update available", body: body,
		choices: []choice{
			{keys: []string{"u", "U"}, label: "update now", style: theme.Good.Bold(true)},
			{keys: []string{"s", "S"}, label: "skip this version", style: theme.Dim},
			{keys: []string{"esc", "n", "q"}, label: "later", style: theme.Dim},
		},
		onChoice: func(m *model, key string) tea.Cmd {
			switch key {
			case "u":
				if m.busy > 0 || m.pending != nil { // flushNotices prevents this; never cut an operation short
					m.setStatus("finish the current operation, then run 'lazynet update'", true)
					return nil
				}
				m.env.updateTo = av.Version
				return tea.Quit
			case "s":
				if err := m.updates.Skip(av.Version); err != nil {
					m.setStatus("could not save your choice: "+err.Error(), true)
					return nil
				}
				m.setStatus("Skipped lazynet "+av.Version+": you'll hear about the next release", false)
			default:
				m.setStatus("Update postponed: run 'lazynet update' whenever you like", false)
			}
			return nil
		},
	}
}

// newWhatsNew shows the release notes of the versions installed since the
// last run, newest first.
func newWhatsNew(notes []update.Note) *modal {
	title := "What's new in lazynet " + notes[0].Version
	return &modal{
		kind: modalNotice, title: title,
		footer: "press any key to close · all releases: lazynet changelog",
		lines: func(width, height int) []string {
			var out []string
			for i, n := range notes {
				if len(notes) > 1 {
					if i > 0 {
						out = append(out, "")
					}
					head := theme.Title.Render(n.Version)
					if n.Date != "" {
						head += theme.Dim.Render("  " + n.Date)
					}
					out = append(out, head)
				}
				out = append(out, render.ReleaseNotes(n.Body, width)...)
			}
			if len(out) > height {
				out = append(out[:height-1], theme.Dim.Render("… more in 'lazynet changelog'"))
			}
			return out
		},
	}
}
