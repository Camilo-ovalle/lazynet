// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed cli.yaml
var baseDefinition []byte

// Dialect is what transport needs to know about the CLIs it may meet. It is
// assembled from the registered platforms (platform.Dialect), so transport
// itself never names a vendor. The safety rules stay here: whatever a
// dialect declares, Show only sends "show …" or one of the exact session
// commands, and changes only go through a confirmed ChangeRequest.
type Dialect struct {
	// FailedWhenContains are output markers meaning the device rejected a
	// command. Show and Apply report them as errors.
	FailedWhenContains []string
	// SessionCommands are the exact commands, besides "show …", that Show
	// accepts: per-session paging settings such as "terminal length 0".
	SessionCommands []string
	// OperationalPrompt accepts a '>' prompt as a platform's working mode
	// (PAN-OS "user@host>"). Such sessions are read-only: Apply refuses
	// them. Any other '>' prompt is Cisco user EXEC: Dial refuses it with
	// ErrNotPrivileged.
	OperationalPrompt func(prompt string) bool
}

// ErrNoDialect is returned by Dial without failure markers: lazynet would not
// notice a device rejecting a command.
var ErrNoDialect = errors.New("transport: no CLI dialect (failure markers) configured")

// sessionVerbs can never start a session command, whatever a platform says.
var sessionVerbs = []string{"configure", "conf", "write", "wr", "copy", "reload", "commit", "delete",
	"erase", "format", "clear", "debug", "request", "enable", "disable", "end", "exit", "quit"}

// normalize collapses whitespace, the form every guard comparison uses.
func normalize(cmd string) string { return strings.Join(strings.Fields(cmd), " ") }

// validate rejects a dialect that would weaken the read-only guard.
func (d Dialect) validate() error {
	if len(d.FailedWhenContains) == 0 {
		return ErrNoDialect
	}
	for _, c := range d.SessionCommands {
		n := normalize(c)
		switch {
		case n == "", n != c, hasControl(c), strings.ContainsAny(c, "|>;&`$"):
			return fmt.Errorf("transport: invalid session command %q", c)
		case strings.HasPrefix(strings.ToLower(n), "show "):
			return fmt.Errorf("transport: session command %q is a show command (already allowed)", c)
		case slices.Contains(sessionVerbs, strings.ToLower(strings.Fields(n)[0])):
			return fmt.Errorf("transport: session command %q is not a session setting", c)
		}
	}
	return nil
}

// definition is the scrapligo platform definition: the base prompts plus
// the dialect's failure markers.
func (d Dialect) definition() ([]byte, error) {
	var def map[string]any
	if err := yaml.Unmarshal(baseDefinition, &def); err != nil {
		return nil, err
	}
	base, ok := def["default"].(map[string]any)
	if !ok {
		return nil, errors.New("transport: cli.yaml has no default section")
	}
	markers := slices.Clone(d.FailedWhenContains)
	slices.Sort(markers)
	base["failed-when-contains"] = slices.Compact(markers)
	return yaml.Marshal(def)
}

func hasControl(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r < 0x20 && r != '\t' || r == 0x7f }) >= 0
}
