// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/app"
	"github.com/Camilo-ovalle/lazynet/internal/audit"
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
	h := &harness{t: t, m: newModel(a, &env{})}
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
