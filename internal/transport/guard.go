// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package transport owns the SSH session to a device and enforces lazynet's
// core safety rule in code: only read-only commands can be sent, except for
// change requests the user has explicitly confirmed. It is vendor-neutral:
// what it needs to know about each CLI arrives as a Dialect.
package transport

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// ErrNotReadOnly is returned when a non read-only command is sent via Show.
var ErrNotReadOnly = errors.New("command rejected: only read-only 'show' commands are allowed")

var (
	// Pipe targets that write files or otherwise have side effects.
	pipeSideEffect = regexp.MustCompile(`(?i)\|\s*(redirect|tee|append|save)\b`)
	// Output redirection used by NX-OS ("show run > bootflash:x").
	redirect = regexp.MustCompile(`>`)
)

// CheckReadOnly returns nil if cmd is safe to run on any device: a single
// "show ..." command (optionally filtered with | include/exclude/begin/section
// or | match/except), or exactly one of session, the per-session paging
// settings declared by the platforms (e.g. "terminal length 0").
func CheckReadOnly(cmd string, session ...string) error {
	if hasControl(cmd) {
		return fmt.Errorf("%w: control characters in %q", ErrNotReadOnly, cmd)
	}
	c := normalize(cmd)
	if c != "" && slices.Contains(session, c) {
		return nil
	}
	if !strings.HasPrefix(strings.ToLower(c), "show ") {
		return fmt.Errorf("%w: %q", ErrNotReadOnly, cmd)
	}
	if pipeSideEffect.MatchString(c) || redirect.MatchString(c) {
		return fmt.Errorf("%w: output redirection in %q", ErrNotReadOnly, cmd)
	}
	return nil
}
