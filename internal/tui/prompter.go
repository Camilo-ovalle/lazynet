package tui

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
)

var errCancelled = errors.New("cancelled")

// promptKind selects which modal a prompt opens.
type promptKind int

const (
	promptUsername promptKind = iota
	promptPassword
	promptHostKey
)

// promptMsg is sent from an operation goroutine to the UI; the goroutine
// blocks on reply until the user answers the modal.
type promptMsg struct {
	kind  promptKind
	title string
	body  string
	reply chan promptReply
}

type promptReply struct {
	value string
	ok    bool
}

// prompter implements app.Prompter by round-tripping through the UI.
type prompter struct{ env *env }

func (p prompter) ask(kind promptKind, title, body string) (promptReply, error) {
	if p.env == nil || p.env.program == nil {
		return promptReply{}, errCancelled
	}
	ch := make(chan promptReply, 1)
	p.env.program.Send(promptMsg{kind: kind, title: title, body: body, reply: ch})
	r := <-ch
	if !r.ok {
		return r, errCancelled
	}
	return r, nil
}

func (p prompter) Username(target string) (string, error) {
	r, err := p.ask(promptUsername, "Username", "SSH username for "+target)
	return r.value, err
}

func (p prompter) Password(user, target string, retry bool) (string, error) {
	body := "Password for " + user + "@" + target
	if retry {
		body = "Authentication failed, try again.\n" + body
	}
	r, err := p.ask(promptPassword, "Password", body)
	return r.value, err
}

func (p prompter) TrustHostKey(addr, keyType, fp string) (bool, error) {
	r, err := p.ask(promptHostKey, "Unknown host key",
		"The authenticity of "+addr+" can't be established\n(first connection).\n\n"+keyType+"\n"+fp+"\n\nTrust this host key and continue?")
	if errors.Is(err, errCancelled) {
		return false, nil
	}
	return r.ok, err
}

// env holds what must be shared across model copies.
type env struct{ program *tea.Program }
