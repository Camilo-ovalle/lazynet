// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type modalKind int

const (
	modalInput modalKind = iota
	modalConfirm
	modalHelp
)

// modal is a dialog drawn over the main view.
type modal struct {
	kind   modalKind
	title  string
	body   string
	danger bool // config change: red border, explicit 'y' required

	labels   []string
	inputs   []textinput.Model
	optional []bool // optional fields don't block Enter-to-submit
	pristine []bool // prefilled and untouched: typing replaces the value
	focus    int

	onSubmit func(m *model, values []string) tea.Cmd
	onAnswer func(m *model, yes bool) tea.Cmd
	onCancel func(m *model) tea.Cmd

	prompt *promptMsg // set when the modal answers a background prompt
}

func newInput(title, body string, fields ...[3]string) *modal {
	md := &modal{kind: modalInput, title: title, body: body}
	for i, f := range fields { // label, placeholder, initial value
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = f[1]
		ti.SetValue(f[2])
		ti.CharLimit = 128
		ti.Width = 36
		ti.Cursor.SetMode(cursor.CursorStatic) // no blink timers: fewer redraws
		if f[2] != "" {
			// Shown as selected, like a pre-selected text field.
			ti.TextStyle = lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#CFE8DD", Dark: "#2A3A40"}).Foreground(theme.Text)
		}
		if i == 0 {
			ti.Focus()
		}
		md.labels = append(md.labels, f[0])
		md.inputs = append(md.inputs, ti)
		md.pristine = append(md.pristine, f[2] != "")
	}
	return md
}

// touch is called before a key reaches field i. A prefilled field that has
// not been edited yet behaves as if its text were selected: typing, pasting or
// deleting replaces it (so a remembered port is not concatenated with the new
// one), while moving the cursor keeps it for editing.
func (md *modal) touch(i int, msg tea.KeyMsg) {
	if i >= len(md.pristine) || !md.pristine[i] {
		return
	}
	switch msg.Type {
	case tea.KeyRunes, tea.KeySpace, tea.KeyBackspace, tea.KeyDelete, tea.KeyCtrlU, tea.KeyCtrlW:
		md.inputs[i].SetValue("")
	}
	md.pristine[i] = false
	md.inputs[i].TextStyle = lipgloss.NewStyle()
}

func newConfirm(title, body string, danger bool, onAnswer func(*model, bool) tea.Cmd) *modal {
	return &modal{kind: modalConfirm, title: title, body: body, danger: danger, onAnswer: onAnswer}
}

func (md *modal) values() []string {
	out := make([]string, len(md.inputs))
	for i, in := range md.inputs {
		out[i] = strings.TrimSpace(in.Value())
	}
	return out
}

func (md *modal) setFocus(i int) {
	md.inputs[md.focus].Blur()
	md.focus = (i + len(md.inputs)) % len(md.inputs)
	md.inputs[md.focus].Focus()
}

// openModal shows md now (or queues background prompts behind it).
func (m *model) openModal(md *modal) { m.modal = md }

// closeModal closes the current dialog and shows the next queued prompt.
func (m *model) closeModal() {
	m.modal = nil
	if len(m.prompts) > 0 {
		next := m.prompts[0]
		m.prompts = m.prompts[1:]
		m.showPrompt(next)
	}
}

func (m *model) showPrompt(p promptMsg) {
	if m.modal != nil {
		m.prompts = append(m.prompts, p)
		return
	}
	var md *modal
	switch p.kind {
	case promptHostKey:
		md = newConfirm(p.title, p.body, false, nil)
	case promptPassword:
		md = newInput(p.title, p.body, [3]string{"Password", "", ""})
		md.inputs[0].EchoMode = textinput.EchoPassword
		md.inputs[0].EchoCharacter = '•'
	default:
		md = newInput(p.title, p.body, [3]string{"Username", "", ""})
	}
	md.prompt = &p
	m.modal = md
}

func (m *model) answerPrompt(value string, ok bool) {
	if m.modal != nil && m.modal.prompt != nil {
		m.modal.prompt.reply <- promptReply{value: value, ok: ok}
	}
}

func (m model) updateModal(msg tea.KeyMsg) (model, tea.Cmd) {
	md := m.modal
	switch md.kind {
	case modalHelp:
		m.closeModal()
		return m, nil
	case modalConfirm:
		switch msg.String() {
		case "y", "Y":
			m.answerPrompt("", true)
			m.closeModal()
			if md.onAnswer != nil {
				return m, md.onAnswer(&m, true)
			}
		case "n", "N", "esc", "q":
			m.answerPrompt("", false)
			m.closeModal()
			if md.onAnswer != nil {
				return m, md.onAnswer(&m, false)
			}
		case "enter":
			if !md.danger { // Enter only defaults to "no" on dangerous prompts
				m.answerPrompt("", false)
				m.closeModal()
				if md.onAnswer != nil {
					return m, md.onAnswer(&m, false)
				}
			}
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.answerPrompt("", false)
		m.closeModal()
		if md.onCancel != nil {
			return m, md.onCancel(&m)
		}
		return m, nil
	case "tab", "down":
		md.setFocus(md.focus + 1)
		return m, nil
	case "shift+tab", "up":
		md.setFocus(md.focus - 1)
		return m, nil
	case "enter":
		// Advance to the next required empty field; submit when none is left.
		for i := md.focus + 1; i < len(md.inputs); i++ {
			if md.inputs[i].Value() == "" && (i >= len(md.optional) || !md.optional[i]) {
				md.setFocus(i)
				return m, nil
			}
		}
		vals := md.values()
		if md.prompt != nil {
			m.answerPrompt(md.inputs[0].Value(), true)
		}
		m.closeModal()
		if md.onSubmit != nil {
			return m, md.onSubmit(&m, vals)
		}
		return m, nil
	}
	md.touch(md.focus, msg)
	var cmd tea.Cmd
	md.inputs[md.focus], cmd = md.inputs[md.focus].Update(msg)
	return m, cmd
}

func (md *modal) view(maxW int) string {
	w := min(66, maxW-4)
	border := theme.Accent
	if md.danger {
		border = theme.Danger
	}
	var b strings.Builder
	b.WriteString(theme.Title.Foreground(border).Render(md.title) + "\n\n")
	if md.body != "" {
		b.WriteString(md.body + "\n")
	}
	switch md.kind {
	case modalInput:
		if md.body != "" {
			b.WriteString("\n")
		}
		lw := 0
		for _, l := range md.labels {
			lw = max(lw, len(l))
		}
		for i, in := range md.inputs {
			lbl := theme.Label.Render(padRight(md.labels[i], lw))
			field := lipgloss.NewStyle().Foreground(theme.Text)
			marker := "  "
			if i == md.focus {
				marker = theme.Title.Render("› ")
			}
			b.WriteString(marker + lbl + "  " + field.Render(in.View()) + "\n")
		}
		b.WriteString("\n" + theme.Dim.Render("enter submit · tab next field · esc cancel"))
	case modalConfirm:
		yes := theme.Good.Render("[y] yes")
		if md.danger {
			yes = theme.Bad.Bold(true).Render("[y] yes, apply")
		}
		b.WriteString("\n" + yes + "    " + theme.Dim.Render("[n] no"))
	case modalHelp:
		b.WriteString("\n" + theme.Dim.Render("press any key to close"))
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).
		Padding(1, 2).Width(w).Render(strings.TrimRight(b.String(), "\n"))
}

func padRight(s string, w int) string {
	if n := lipgloss.Width(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

const helpText = `Navigation
  ↑/↓ j/k     move / scroll          tab / h l   switch pane
  1-4 [ ]     switch tab             mouse       click, wheel
  enter       connect / load         r           refresh tab

Actions
  p           query an interface     b           take a backup
  v           change access VLAN     a           connect to an IP
  w           toggle raw output

Logs tab
  f           date/time range        /           text filter
  s           cycle severity filter

Backups tab
  ↑/↓         select a backup        enter       diff vs previous

  ?  help       q / ctrl+c  quit`
