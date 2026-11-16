// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package platform

import (
	"strings"
	"testing"
)

type infoOnly struct{}

func (infoOnly) Info(string) (Info, error) { return Info{}, nil }

type withLogs struct{ infoOnly }

func (withLogs) Logs() (LogResult, error) { return LogResult{}, nil }

func def(id ID, newDrv func(Shower) Driver, aliases ...string) Definition {
	return Definition{
		ID: id, Name: strings.ToUpper(string(id)), Aliases: aliases,
		Badge:   Badge{Text: "X"},
		Probes:  []Probe{{Command: "show version", Match: func(string) bool { return false }}},
		Session: Session{FailedWhenContains: []string{"% Invalid"}},
		New:     newDrv,
	}
}

func mustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected a panic", name)
		}
	}()
	f()
}

func TestRegistry(t *testing.T) {
	r := &registry{}
	r.register(def("beta", func(Shower) Driver { return infoOnly{} }))
	a := def("alpha", func(Shower) Driver { return withLogs{} }, "a1")
	a.Session.Setup = []string{"terminal length 0"}
	a.Session.OperationalPrompt = func(p string) bool { return p == "x>" }
	r.register(a)

	if got := r.namesText(); got != "alpha or beta" {
		t.Errorf("namesText = %q", got)
	}
	if id, err := r.parse(" A1 "); err != nil || id != "alpha" {
		t.Errorf("parse alias = %q, %v", id, err)
	}
	if _, err := r.parse("gamma"); err == nil || !strings.Contains(err.Error(), "use alpha or beta") {
		t.Errorf("parse unknown = %v", err)
	}
	mustPanic(t, "duplicate ID", func() { r.register(def("beta", func(Shower) Driver { return infoOnly{} })) })
	mustPanic(t, "duplicate alias", func() { r.register(def("gamma", func(Shower) Driver { return infoOnly{} }, "a1")) })
	mustPanic(t, "upper-case ID", func() { r.register(def("Gamma", func(Shower) Driver { return infoOnly{} })) })
	mustPanic(t, "no driver", func() { r.register(def("delta", nil)) })

	d := r.dialect()
	if len(d.SessionCommands) != 1 || len(d.FailedWhenContains) != 1 || d.OperationalPrompt == nil || !d.OperationalPrompt("x>") || d.OperationalPrompt("y>") {
		t.Errorf("dialect = %+v", d)
	}
}

func TestCapabilitiesFollowTheDriver(t *testing.T) {
	d := def("alpha", func(Shower) Driver { return withLogs{} })
	if !d.Supports(CapLogs) || d.Supports(CapInterfaces) || d.Supports(CapVlanChange) {
		t.Fatalf("capabilities = %v", d.Capabilities())
	}
}
