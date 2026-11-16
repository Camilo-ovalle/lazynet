// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"slices"

	"github.com/Camilo-ovalle/lazynet/internal/transport"
)

// Dialect merges what every registered platform declares about its CLI into
// the transport.Dialect used to open sessions: failure markers, the exact
// session setup commands Show may send, and the operational '>' prompts.
// One session must handle any platform, because the platform is only known
// after detection.
func Dialect() transport.Dialect { return reg.dialect() }

func (r *registry) dialect() transport.Dialect {
	var d transport.Dialect
	var operational []func(string) bool
	for _, def := range r.all() {
		d.FailedWhenContains = append(d.FailedWhenContains, def.Session.FailedWhenContains...)
		d.SessionCommands = append(d.SessionCommands, def.Session.Setup...)
		if def.Session.OperationalPrompt != nil {
			operational = append(operational, def.Session.OperationalPrompt)
		}
	}
	slices.Sort(d.FailedWhenContains)
	d.FailedWhenContains = slices.Compact(d.FailedWhenContains)
	slices.Sort(d.SessionCommands)
	d.SessionCommands = slices.Compact(d.SessionCommands)
	if len(operational) > 0 {
		d.OperationalPrompt = func(prompt string) bool {
			for _, f := range operational {
				if f(prompt) {
					return true
				}
			}
			return false
		}
	}
	return d
}
