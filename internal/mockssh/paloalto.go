// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package mockssh

import "strings"

// Palo Alto (PAN-OS) operational mode: "user@host>" prompt, paging control,
// the show commands lazynet uses (from fixtures, filterable with "| match"),
// and "Invalid syntax." for anything else, like the real CLI.
func init() { register("paloalto", panos{}) }

type panos struct{}

func (panos) setup(*Device) {}

func (panos) prompt(s *session) string {
	user := s.user
	if user == "" {
		user = "admin"
	}
	return user + "@" + s.d.Hostname + ">"
}

func (panos) exec(s *session, line string) (string, bool) {
	d := s.d
	c := strings.Join(strings.Fields(line), " ")
	switch {
	case c == "exit" || c == "quit":
		return "", true
	case c == "set cli pager off" || c == "set cli pager on":
		return "", false
	case c == "show clock":
		return d.Now().UTC().Format("Mon Jan 2 15:04:05 MST 2006"), false
	}
	base, expr, filtered := strings.Cut(c, " | match ")
	out, ok := d.static[strings.ToLower(base)]
	if !ok {
		return "Invalid syntax.", false
	}
	out = d.render(out, d.Now())
	if !filtered {
		return out, false
	}
	var keep []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, expr) {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n"), false
}
