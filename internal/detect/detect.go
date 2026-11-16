// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package detect identifies the platform of a freshly opened session. It owns
// the detection pipeline and nothing else: the rules themselves (probes) are
// declared by each platform package, so detection rules for different vendors
// never live in the same function.
//
// Pipeline:
//  1. A platform pinned in the inventory wins; no command is sent.
//  2. Otherwise every registered probe runs by ascending priority (ties keep
//     the platforms' display order). A probe can require a prompt shape, so
//     a vendor's command never reaches another vendor's CLI. Probes sharing
//     a command run it once. The first match wins.
package detect

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/transport"
)

// Session is an open connection: its login prompt and read-only commands
// (*transport.Conn).
type Session interface {
	platform.Shower
	Prompt() string
}

// ErrUnrecognised means no registered platform matched the device.
var ErrUnrecognised = errors.New("unrecognised device")

// Identify returns the platform of the device behind s. pinned is the
// inventory's platform field (empty means auto-detect).
func Identify(s Session, pinned string) (platform.Definition, error) {
	id, err := platform.Parse(pinned)
	if err != nil {
		return platform.Definition{}, err
	}
	if id != platform.Unknown {
		def, _ := platform.Lookup(id) // Parse only returns registered IDs
		return def, nil
	}
	return Run(s, platform.All())
}

// Run evaluates the probes of defs against s.
func Run(s Session, defs []platform.Definition) (platform.Definition, error) {
	type candidate struct {
		def   platform.Definition
		probe platform.Probe
	}
	var cands []candidate
	for _, d := range defs { // defs are in display order; the sort is stable
		for _, p := range d.Probes {
			cands = append(cands, candidate{d, p})
		}
	}
	slices.SortStableFunc(cands, func(a, b candidate) int { return a.probe.Priority - b.probe.Priority })

	type result struct {
		out string
		err error
	}
	outputs := map[string]result{}
	prompt := s.Prompt()
	var rejected error
	for _, c := range cands {
		if c.probe.Prompt != nil && !c.probe.Prompt(prompt) {
			continue
		}
		r, done := outputs[c.probe.Command]
		if !done {
			out, err := s.Show(c.probe.Command)
			r = result{out, err}
			outputs[c.probe.Command] = r
		}
		switch {
		case errors.Is(r.err, transport.ErrRejected):
			// Another vendor's command: keep trying the other probes.
			if rejected == nil {
				rejected = r.err
			}
		case r.err != nil:
			return platform.Definition{}, fmt.Errorf("detecting platform: %w", r.err)
		case c.probe.Match(r.out):
			return c.def, nil
		}
	}
	if rejected != nil {
		return platform.Definition{}, fmt.Errorf("%w: no platform matched (%v); supported: %s", ErrUnrecognised, rejected, platform.Names())
	}
	return platform.Definition{}, fmt.Errorf("%w: no platform matched the device output; supported: %s", ErrUnrecognised, platform.Names())
}
