// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/app"
	"github.com/Camilo-ovalle/lazynet/internal/audit"
	"github.com/Camilo-ovalle/lazynet/internal/update"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// harness drives the model synchronously: every command is executed and its
// message fed back, except spinner ticks (which only sleep).
type harness struct {
	t *testing.T
	m tea.Model
}

func (h *harness) send(msg tea.Msg) {
	h.t.Helper()
	queue := []tea.Msg{msg}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 200 {
			h.t.Fatal("message loop did not settle")
		}
		next := queue[0]
		queue = queue[1:]
		var cmd tea.Cmd
		h.m, cmd = h.m.Update(next)
		queue = append(queue, run(cmd)...)
	}
}

// run executes a command. Device operations against the demo devices finish
// in milliseconds; timer-based commands (cursor blink, spinner) sleep, so
// anything slower than the cutoff is treated as a timer and dropped.
func run(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-ch:
	case <-time.After(400 * time.Millisecond):
		return nil
	}
	switch msg := msg.(type) {
	case nil, spinner.TickMsg:
		return nil
	case tea.BatchMsg:
		var out []tea.Msg
		for _, c := range msg {
			out = append(out, run(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func (h *harness) key(s string) {
	h.t.Helper()
	switch s {
	case "enter":
		h.send(tea.KeyMsg{Type: tea.KeyEnter})
	case "tab":
		h.send(tea.KeyMsg{Type: tea.KeyTab})
	case "esc":
		h.send(tea.KeyMsg{Type: tea.KeyEsc})
	case "down":
		h.send(tea.KeyMsg{Type: tea.KeyDown})
	default:
		h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	}
}

func (h *harness) screen() string { return ansi.Strip(h.m.View()) }

func (h *harness) expect(subs ...string) {
	h.t.Helper()
	sc := h.screen()
	for _, s := range subs {
		if !strings.Contains(sc, s) {
			h.t.Fatalf("screen does not contain %q:\n%s", s, sc)
		}
	}
}

func (h *harness) status() string { return h.m.(model).status }

func newHarness(t *testing.T) (*harness, *app.App) {
	a, stop, err := app.StartDemo()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	h := &harness{t: t, m: newModel(a, &env{}, Options{})}
	h.send(tea.WindowSizeMsg{Width: 130, Height: 38})
	return h, a
}

func TestTUIBrowse(t *testing.T) {
	h, _ := newHarness(t)
	h.expect("Devices", "core-sw1", "dc-n9k-1", "fw-edge", "1 Info", "4 Backups", "CORE")

	h.key("enter")
	h.expect("C9300-48P", "FCW2318L0AB", "IOS-XE 17.09.04a")

	h.key("2") // opens the interface prompt
	h.expect("Query interface")
	h.key("1/0/25")
	h.key("enter")
	h.expect("GigabitEthernet1/0/25", "Camera-Parking", "notconnect")

	h.key("3")
	h.expect("LINK-3-UPDOWN", "Showing")
	h.key("s") // warning and worse
	h.expect("warning and worse")
	h.key("/")
	h.key("MACFLAP")
	h.key("enter")
	h.expect("Showing   1 of")

	h.key("?")
	h.expect("lazynet keys")
	h.key("x")
	if strings.Contains(h.screen(), "lazynet keys") {
		t.Fatal("help should close on any key")
	}
	if os.Getenv("LAZYNET_SNAPSHOT") != "" {
		t.Log("\n" + h.screen())
	}
}

func TestTUIMouse(t *testing.T) {
	h, _ := newHarness(t)
	m := h.m.(model)
	// Find the screen row of dc-n9k-1 and click it.
	y := -1
	for i, r := range m.rows {
		if r.device != nil && r.device.Name == "dc-n9k-1" {
			y = i - m.listOffset + 1
		}
	}
	h.send(tea.MouseMsg{X: 5, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	h.expect("Nexus9000 C93180YC-FX", "NX-OS 10.2(6)")
	// Click the "3 Logs" tab in the top border.
	m = h.m.(model)
	h.send(tea.MouseMsg{X: m.tabRanges()[tabLogs][0] + 1, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	h.expect("ETHPORT-5-IF_UP")
	h.send(tea.MouseMsg{X: 80, Y: 10, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
}

func readAudit(t *testing.T, a *app.App) []audit.Record {
	f, err := os.Open(a.Audit.Path())
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []audit.Record
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r audit.Record
		_ = json.Unmarshal(sc.Bytes(), &r)
		out = append(out, r)
	}
	return out
}

func TestTUIVlanChange(t *testing.T) {
	h, a := newHarness(t)
	h.key("enter") // connect first, as a user would: the Info tab is loaded
	h.expect("C9300-48P")
	vlanForm := func(port, vlan string) {
		h.key("v")
		h.expect("Change access VLAN")
		h.key(port)
		h.key("tab")
		h.key(vlan)
		h.key("enter")
	}

	// Trunk port is refused before anything is shown.
	vlanForm("te1/1/1", "20")
	if !strings.Contains(h.status(), "trunk") {
		t.Fatalf("status: %q", h.status())
	}

	// Decline: nothing sent, audited as declined.
	vlanForm("1/0/7", "20")
	h.expect("Confirm configuration change", "configure terminal", "interface GigabitEthernet1/0/7", "switchport access vlan 20", "[y] yes, apply")
	h.key("enter") // Enter must NOT approve a dangerous change
	h.expect("Confirm configuration change")
	h.key("n")
	if !strings.Contains(h.status(), "Aborted") {
		t.Fatalf("status: %q", h.status())
	}

	// Accept and save.
	vlanForm("1/0/7", "20")
	h.key("y")
	h.expect("Save configuration?", "write memory")
	h.key("y")
	if !strings.Contains(h.status(), "configuration saved") {
		t.Fatalf("status: %q", h.status())
	}
	// Regression: device info must survive the change (it used to reset to
	// "enter connect and load device information"), and the changed port is
	// re-read from the device.
	h.expect("C9300-48P", "FCW2318L0AB")
	h.key("2")
	h.key("esc") // close the interface prompt; the reloaded port is shown
	h.expect("GigabitEthernet1/0/7", "Access VLAN  20")

	// Verify on the device through the UI.
	h.key("p")
	h.send(tea.KeyMsg{Type: tea.KeyCtrlU}) // clear the prefilled field
	h.key("1/0/7")
	h.key("enter")
	h.expect("Access VLAN  20")

	recs := readAudit(t, a)
	var results []string
	for _, r := range recs {
		results = append(results, r.Action+":"+r.Result)
	}
	if strings.Join(results, ",") != "vlan-change:declined,vlan-change:applied,save-config:applied" {
		t.Fatalf("audit = %v", results)
	}
}

func TestTUIBackups(t *testing.T) {
	h, _ := newHarness(t)
	h.key("j") // access-sw2 → core-sw1
	h.key("4")
	h.expect("No backups of core-sw1 yet")
	h.key("b")
	h.expect("latest")
	if !strings.Contains(h.status(), "backup of core-sw1 saved") {
		t.Fatalf("status: %q", h.status())
	}
	h.key("b")
	if !strings.Contains(h.status(), "unchanged") {
		t.Fatalf("second backup should be unchanged: %q", h.status())
	}
	h.key("l") // focus detail
	h.key("enter")
	h.expect("hostname core-sw1") // first backup: full content shown
}

func TestTUIAdhocAndSmallTerminal(t *testing.T) {
	h, a := newHarness(t)
	h.key("a")
	h.expect("Connect to a device")
	h.key(a.Inv.Devices[3].Addr()) // fw-edge by address resolves to the inventory entry
	h.key("enter")
	h.expect("ASA5516", "JAD21430ABC")
	h.send(tea.WindowSizeMsg{Width: 40, Height: 10})
	h.expect("resize your terminal")
}

// A prefilled field behaves as selected: typing replaces the remembered value
// instead of appending to it ("1/0/25" + "1/0/7" used to become "1/0/251/0/7").
func TestTUIPrefilledFieldIsReplaced(t *testing.T) {
	h, _ := newHarness(t)
	h.key("enter")
	h.key("2")
	h.key("1/0/25")
	h.key("enter")
	h.expect("GigabitEthernet1/0/25")
	h.key("v") // the port field is prefilled with 1/0/25
	h.key("1/0/7")
	h.key("tab")
	h.key("20")
	h.key("enter")
	h.expect("Confirm configuration change", "interface GigabitEthernet1/0/7")
	h.key("n")

	// Moving the cursor first keeps the value for editing.
	h.key("p")
	h.send(tea.KeyMsg{Type: tea.KeyEnd})
	h.send(tea.KeyMsg{Type: tea.KeyBackspace})
	h.key("6")
	h.key("enter")
	h.expect("GigabitEthernet1/0/26")
}

func TestTUIPaloAlto(t *testing.T) {
	h, _ := newHarness(t)
	for range 4 { // access-sw2 -> core-sw1 -> dc-n9k-1 -> fw-edge -> pa-fw1
		h.key("j")
	}
	h.key("enter")
	h.expect("pa-fw1", "PAN", "PA-440", "PAN-OS 11.1.4-h7", "024101001234")
	if strings.Contains(h.screen(), "VLAN change") {
		t.Fatal("VLAN change must not be offered for a Palo Alto")
	}
	h.key("4")
	h.key("b")
	if !strings.Contains(h.status(), "backup of pa-fw1 saved") {
		t.Fatalf("status: %q", h.status())
	}
	h.key("3")
	h.expect("Palo Alto: logs not supported yet (available: device info, configuration backups)")
}

// Features follow the platform's capabilities: nothing is offered (or sent)
// that the driver does not implement.
func TestTUICapabilities(t *testing.T) {
	h, _ := newHarness(t)
	for range 2 { // access-sw2 -> core-sw1 -> dc-n9k-1
		h.key("j")
	}
	h.key("enter")
	h.expect("dc-n9k-1", "NX-OS", "p interface", "b backup")
	if strings.Contains(h.screen(), "VLAN change") {
		t.Fatal("VLAN change must not be offered on a Nexus")
	}
	h.key("v")
	if !strings.Contains(h.status(), "VLAN changes not supported yet") {
		t.Fatalf("status: %q", h.status())
	}
	if h.m.(model).modal != nil {
		t.Fatal("no VLAN form must open on a Nexus")
	}
	h.key("h") // back to the device list
	h.key("j") // fw-edge
	h.key("j") // pa-fw1
	h.key("enter")
	h.key("p")
	h.expect("Palo Alto: interface details not supported yet")
	if h.m.(model).modal != nil {
		t.Fatal("no interface prompt must open on a Palo Alto")
	}
}

type fakeUpdates struct {
	notes   []update.Note
	avail   update.Available
	skipped string
	checks  int
}

func (f *fakeUpdates) WhatsNew() []update.Note {
	n := f.notes
	f.notes = nil
	return n
}

func (f *fakeUpdates) Check(context.Context) (update.Available, error) {
	f.checks++
	return f.avail, nil
}

func (f *fakeUpdates) Skip(v string) error { f.skipped = v; return nil }

func newUpdateHarness(t *testing.T, f *fakeUpdates) (*harness, *env) {
	a, stop, err := app.StartDemo()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	e := &env{}
	h := &harness{t: t, m: newModel(a, e, Options{Updates: f})}
	h.send(tea.WindowSizeMsg{Width: 130, Height: 38})
	return h, e
}

// init runs the model's startup commands, as Bubble Tea does.
func (h *harness) init() {
	h.t.Helper()
	for _, msg := range run(h.m.Init()) {
		h.send(msg)
	}
}

var available = update.Available{Current: "0.2.0", Version: "0.3.0", URL: "https://github.com/Camilo-ovalle/lazynet/releases/tag/v0.3.0"}

func TestTUIWhatsNew(t *testing.T) {
	f := &fakeUpdates{notes: []update.Note{
		{Version: "0.3.0", Date: "2026-12-01", Body: "### Added\n\n- `lazynet update` installs\n  new releases."},
		{Version: "0.2.0", Date: "2026-11-20", Body: "### Fixed\n\n- Faster backups."},
	}}
	h, _ := newUpdateHarness(t, f)
	h.expect("What's new in lazynet 0.3.0", "0.3.0  2026-12-01", "Added", "• lazynet update installs new releases.",
		"0.2.0  2026-11-20", "• Faster backups.", "all releases: lazynet changelog")
	h.key("x")
	if strings.Contains(h.screen(), "What's new") {
		t.Fatal("the release notes should close on any key")
	}
	h.init() // nothing newer: no banner
	if f.checks != 1 || h.m.(model).modal != nil {
		t.Fatalf("checks = %d, modal = %v", f.checks, h.m.(model).modal)
	}
	if os.Getenv("LAZYNET_SNAPSHOT") != "" {
		t.Log("\n" + h.screen())
	}
}

// Long notes are cut to fit the terminal instead of overflowing it.
func TestTUIWhatsNewFitsTheScreen(t *testing.T) {
	body := "### Added\n"
	for i := range 60 {
		body += fmt.Sprintf("\n- Item number %d with enough words to need wrapping inside the dialog box.", i)
	}
	h, _ := newUpdateHarness(t, &fakeUpdates{notes: []update.Note{{Version: "0.3.0", Body: body}}})
	h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.expect("What's new in lazynet 0.3.0", "Item number 0", "… more in 'lazynet changelog'", "press any key")
	if lines := strings.Split(h.screen(), "\n"); !strings.HasPrefix(lines[0], "╭─ Devices") {
		t.Fatalf("the dialog must leave the frame visible:\n%s", h.screen())
	}
	if n := len(strings.Split(h.screen(), "\n")); n != 24 {
		t.Fatalf("screen has %d lines", n)
	}
}

func TestTUIUpdateBanner(t *testing.T) {
	t.Run("skip", func(t *testing.T) {
		f := &fakeUpdates{avail: available}
		h, e := newUpdateHarness(t, f)
		h.init()
		h.expect("Update available", "lazynet 0.3.0 is available. You have 0.2.0.", "releases/tag/v0.3.0",
			"[u] update now", "[s] skip this version", "[esc] later")
		h.key("enter") // Enter chooses nothing
		h.expect("Update available")
		h.key("s")
		if f.skipped != "0.3.0" || e.updateTo != "" || h.m.(model).modal != nil {
			t.Fatalf("skipped = %q, updateTo = %q", f.skipped, e.updateTo)
		}
		if !strings.Contains(h.status(), "Skipped lazynet 0.3.0") {
			t.Fatalf("status: %q", h.status())
		}
	})
	t.Run("later", func(t *testing.T) {
		f := &fakeUpdates{avail: available}
		h, e := newUpdateHarness(t, f)
		h.init()
		h.key("esc")
		if f.skipped != "" || e.updateTo != "" || !strings.Contains(h.status(), "run 'lazynet update'") {
			t.Fatalf("skipped = %q, updateTo = %q, status %q", f.skipped, e.updateTo, h.status())
		}
	})
	t.Run("update", func(t *testing.T) {
		h, e := newUpdateHarness(t, &fakeUpdates{avail: available})
		h.init()
		var cmd tea.Cmd
		h.m, cmd = h.m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
		if e.updateTo != "0.3.0" {
			t.Fatalf("updateTo = %q", e.updateTo)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatal("choosing update must close the UI so the update runs in the terminal")
		}
	})
	t.Run("after the release notes", func(t *testing.T) {
		h, _ := newUpdateHarness(t, &fakeUpdates{avail: available, notes: []update.Note{{Version: "0.2.0", Body: "- two"}}})
		h.init()
		h.expect("What's new in lazynet 0.2.0")
		h.key("x")
		h.expect("Update available")
	})
}

// The banner waits until the user is done: it never covers a dialog or
// interrupts a VLAN change.
func TestTUIUpdateBannerWaits(t *testing.T) {
	h, e := newUpdateHarness(t, &fakeUpdates{})
	h.key("enter")
	h.expect("C9300-48P")
	h.key("v")
	h.send(updateMsg{avail: available})
	h.expect("Change access VLAN")
	h.key("1/0/7")
	h.key("tab")
	h.key("20")
	h.key("enter")
	h.expect("Confirm configuration change", "switchport access vlan 20")
	if strings.Contains(h.screen(), "Update available") {
		t.Fatal("the banner must not cover the change confirmation")
	}
	h.key("n")
	h.expect("Update available")
	if !strings.Contains(h.status(), "Aborted") {
		t.Fatalf("status: %q", h.status())
	}
	h.key("u")
	if e.updateTo != "0.3.0" {
		t.Fatalf("updateTo = %q", e.updateTo)
	}
}

func TestTUIWithoutUpdates(t *testing.T) {
	h, _ := newHarness(t)
	if h.m.Init() == nil {
		t.Fatal("Init must still start the spinner")
	}
	h.init()
	if h.m.(model).modal != nil {
		t.Fatal("no banner without an update checker (demo mode, dev builds)")
	}
}

// Field regression: "dynamic auto" ports (the Catalyst default) were shown
// as trunks and their VLAN couldn't be changed. They show their real mode,
// and the change sends only the access VLAN, after a DTP warning.
func TestTUIDynamicPort(t *testing.T) {
	h, a := newHarness(t)
	h.key("enter")
	h.expect("C9300-48P")
	h.key("2")
	h.key("1/0/41")
	h.key("enter")
	h.expect("GigabitEthernet1/0/41", "dynamic auto · operational: down", "Access VLAN  999 PARKING")
	if strings.Contains(h.screen(), "Mode         trunk") {
		t.Fatal("a dynamic port must not be shown as a trunk")
	}

	h.key("v")
	h.send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.key("1/0/41")
	h.key("tab")
	h.key("110")
	h.key("enter")
	h.expect("Confirm configuration change", "Mode     dynamic auto, operational: down",
		"switchport access vlan 110", "is in dynamic auto mode: it keeps")
	if strings.Contains(h.screen(), "switchport mode") {
		t.Fatal("the port mode must not be changed")
	}
	h.key("y")
	h.expect("Save configuration?")
	h.key("n")
	h.expect("Access VLAN  110 PRINTERS", "dynamic auto")

	// A dynamic port that negotiated a trunk is refused.
	h.key("v")
	h.send(tea.KeyMsg{Type: tea.KeyCtrlU})
	h.key("1/0/39")
	h.key("tab")
	h.key("110")
	h.key("enter")
	if !strings.Contains(h.status(), "operating as one") || h.m.(model).modal != nil {
		t.Fatalf("status: %q", h.status())
	}
	var results []string
	for _, r := range readAudit(t, a) {
		results = append(results, r.Action+":"+r.Result+":"+r.Port)
	}
	if strings.Join(results, ",") != "vlan-change:applied:GigabitEthernet1/0/41" {
		t.Fatalf("audit = %v", results)
	}
}
