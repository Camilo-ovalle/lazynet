// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package transport owns the SSH session to a device and enforces lazynet's
// core safety rule in code: only read-only commands can be sent, except for
// change requests the user has explicitly confirmed.
package transport

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrNotReadOnly is returned when a non read-only command is sent via Show.
var ErrNotReadOnly = errors.New("command rejected: only read-only 'show' commands are allowed")

var (
	terminalCmd = regexp.MustCompile(`^terminal (length 0|pager 0|width \d{1,3})$`)
	// PAN-OS session setting: disables paging for this CLI session only (it is
	// an operational-mode command and never touches the configuration).
	panosPagerOff = "set cli pager off"
	// Pipe targets that write files or otherwise have side effects.
	pipeSideEffect = regexp.MustCompile(`(?i)\|\s*(redirect|tee|append|save)\b`)
	// Output redirection used by NX-OS ("show run > bootflash:x").
	nxRedirect = regexp.MustCompile(`>`)
)

// CheckReadOnly returns nil if cmd is safe to run on any device: a single
// "show ..." command (optionally filtered with | include/exclude/begin/section
// or PAN-OS | match/except) or a paging setting for the current session only
// (Cisco "terminal ...", PAN-OS "set cli pager off").
func CheckReadOnly(cmd string) error {
	if strings.ContainsAny(cmd, "\r\n\x00") || strings.IndexFunc(cmd, func(r rune) bool { return r < 0x20 && r != '\t' }) >= 0 {
		return fmt.Errorf("%w: control characters in %q", ErrNotReadOnly, cmd)
	}
	c := strings.Join(strings.Fields(cmd), " ")
	if terminalCmd.MatchString(c) || c == panosPagerOff {
		return nil
	}
	if !strings.HasPrefix(strings.ToLower(c), "show ") {
		return fmt.Errorf("%w: %q", ErrNotReadOnly, cmd)
	}
	if pipeSideEffect.MatchString(c) || nxRedirect.MatchString(c) {
		return fmt.Errorf("%w: output redirection in %q", ErrNotReadOnly, cmd)
	}
	return nil
}
