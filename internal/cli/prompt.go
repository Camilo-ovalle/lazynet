// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/Camilo-ovalle/lazynet/internal/theme"
	"golang.org/x/term"
)

// termPrompter asks questions on the controlling terminal (like sudo/ssh),
// so prompts work even when stdout/stdin are redirected.
type termPrompter struct {
	once   sync.Once
	in     *os.File
	out    io.Writer
	reader *bufio.Reader
}

func newPrompter() *termPrompter { return &termPrompter{out: os.Stderr} }

func (p *termPrompter) tty() *os.File {
	p.once.Do(func() {
		name := "/dev/tty"
		if runtime.GOOS == "windows" {
			name = "CONIN$"
		}
		if f, err := os.OpenFile(name, os.O_RDWR, 0); err == nil {
			p.in, p.out = f, f
		} else {
			p.in = os.Stdin
		}
		p.reader = bufio.NewReader(p.in)
	})
	return p.in
}

func (p *termPrompter) line(prompt string) (string, error) {
	p.tty()
	fmt.Fprint(p.out, prompt)
	s, err := p.reader.ReadString('\n')
	if err != nil && s == "" {
		return "", fmt.Errorf("no answer (stdin closed): %w", err)
	}
	return strings.TrimSpace(s), nil
}

func (p *termPrompter) Username(target string) (string, error) {
	u, err := p.line(fmt.Sprintf("Username for %s: ", target))
	if err == nil && u == "" {
		err = fmt.Errorf("username required")
	}
	return u, err
}

func (p *termPrompter) Password(user, target string, retry bool) (string, error) {
	f := p.tty()
	if retry {
		fmt.Fprintln(p.out, theme.Bad.Render("Authentication failed, try again."))
	}
	fmt.Fprintf(p.out, "Password for %s@%s: ", user, target)
	if term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(p.out)
		return string(b), err
	}
	return p.line("")
}

func (p *termPrompter) TrustHostKey(addr, keyType, fp string) (bool, error) {
	fmt.Fprintf(p.out, "%s\n  %s %s\n",
		theme.Caution.Render("The authenticity of "+addr+" can't be established (first connection)."), keyType, fp)
	return p.confirm("Trust this host key and continue?")
}

// confirm asks a yes/no question; anything but y/yes is "no".
func (p *termPrompter) confirm(q string) (bool, error) {
	a, err := p.line(q + " [y/N]: ")
	if err != nil {
		return false, err
	}
	a = strings.ToLower(a)
	return a == "y" || a == "yes", nil
}
