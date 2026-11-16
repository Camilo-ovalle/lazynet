// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package detect

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/transport"
)

// fake answers from a table and counts the commands it received.
type fake struct {
	prompt string
	out    map[string]string
	err    map[string]error
	sent   []string
}

func (f *fake) Prompt() string { return f.prompt }
func (f *fake) Show(cmd string) (string, error) {
	f.sent = append(f.sent, cmd)
	return f.out[cmd], f.err[cmd]
}

func def(id string, order int, probes ...platform.Probe) platform.Definition {
	return platform.Definition{ID: platform.ID(id), Order: order, Probes: probes}
}

func contains(s string) func(string) bool {
	return func(out string) bool { return strings.Contains(out, s) }
}

func TestRunPriorityAndSharedCommands(t *testing.T) {
	f := &fake{prompt: "sw1#", out: map[string]string{"show version": "Cisco IOS XE Software, NX-OS-like"}}
	defs := []platform.Definition{
		// Broad match, declared first but with a later priority.
		def("broad", 1, platform.Probe{Priority: 30, Command: "show version", Match: contains("Cisco")}),
		def("specific", 2, platform.Probe{Priority: 10, Command: "show version", Match: contains("NX-OS")}),
	}
	got, err := Run(f, defs)
	if err != nil || got.ID != "specific" {
		t.Fatalf("got %q, %v", got.ID, err)
	}
	if len(f.sent) != 1 {
		t.Fatalf("a shared probe command must run once, sent %v", f.sent)
	}
}

func TestRunPromptFilterAndRejections(t *testing.T) {
	f := &fake{
		prompt: "admin@fw1>",
		out:    map[string]string{"show system info": "sw-version: 11.1"},
		err:    map[string]error{"show version": fmt.Errorf("show version: %w", transport.ErrRejected)},
	}
	defs := []platform.Definition{
		def("cisco", 1, platform.Probe{Priority: 1, Prompt: func(p string) bool { return strings.HasSuffix(p, "#") }, Command: "show version", Match: contains("Cisco")}),
		def("other", 2, platform.Probe{Priority: 5, Command: "show version", Match: contains("x")}),
		def("pan", 3, platform.Probe{Priority: 10, Command: "show system info", Match: contains("sw-version:")}),
	}
	got, err := Run(f, defs)
	if err != nil || got.ID != "pan" {
		t.Fatalf("got %q, %v", got.ID, err)
	}
	// "cisco" was skipped by its prompt filter; "other" was rejected and the
	// detector moved on.
	if strings.Join(f.sent, "|") != "show version|show system info" {
		t.Fatalf("sent %v", f.sent)
	}
}

func TestRunErrors(t *testing.T) {
	defs := []platform.Definition{def("a", 1, platform.Probe{Command: "show version", Match: contains("A")})}
	if _, err := Run(&fake{prompt: "x#", out: map[string]string{"show version": "B"}}, defs); !errors.Is(err, ErrUnrecognised) {
		t.Fatalf("no match: %v", err)
	}
	timeout := errors.New("i/o timeout")
	f := &fake{prompt: "x#", err: map[string]error{"show version": timeout}}
	if _, err := Run(f, defs); !errors.Is(err, timeout) || errors.Is(err, ErrUnrecognised) {
		t.Fatalf("a transport error must stop detection: %v", err)
	}
}
