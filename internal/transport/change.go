// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"errors"
	"slices"
	"strings"
)

// ErrNotConfirmed is returned when an unconfirmed ChangeRequest is applied.
var ErrNotConfirmed = errors.New("change request was not confirmed by the user")

// ChangeKind selects how a change is sent to the device.
type ChangeKind int

const (
	// ConfigLines are sent inside configuration mode (configure terminal ... end).
	ConfigLines ChangeKind = iota
	// ExecCommand is a single privileged exec command (e.g. write memory).
	ExecCommand
)

// ChangeRequest is the only way to send a configuration-changing command.
// It can only be applied after Confirm is called with the exact command list
// that was displayed to the user, guaranteeing that what was shown is
// precisely what gets sent.
type ChangeRequest struct {
	kind      ChangeKind
	lines     []string
	confirmed bool
}

// NewConfigChange builds a change sent in configuration mode.
func NewConfigChange(lines ...string) *ChangeRequest {
	return &ChangeRequest{kind: ConfigLines, lines: slices.Clone(lines)}
}

// NewExecChange builds a single privileged-exec change (e.g. "write memory").
func NewExecChange(cmd string) *ChangeRequest {
	return &ChangeRequest{kind: ExecCommand, lines: []string{cmd}}
}

// Display returns every line that will be typed on the device, including
// entering and leaving configuration mode. This is what must be shown to the
// user before asking for confirmation.
func (c *ChangeRequest) Display() []string {
	if c.kind == ExecCommand {
		return slices.Clone(c.lines)
	}
	out := []string{"configure terminal"}
	out = append(out, c.lines...)
	return append(out, "end")
}

// Confirm marks the request as approved. shown must equal Display(); a
// mismatch means the UI showed something different and the request stays
// unconfirmed.
func (c *ChangeRequest) Confirm(shown []string) bool {
	c.confirmed = slices.Equal(shown, c.Display())
	return c.confirmed
}

// Confirmed reports whether Confirm succeeded.
func (c *ChangeRequest) Confirmed() bool { return c.confirmed }

// String renders the displayed commands one per line.
func (c *ChangeRequest) String() string { return strings.Join(c.Display(), "\n") }
