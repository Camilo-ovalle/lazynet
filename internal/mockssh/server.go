// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package mockssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

// Server is a running fake SSH endpoint for one Device.
type Server struct {
	Device *Device
	ln     net.Listener
	cfg    *ssh.ServerConfig
	wg     sync.WaitGroup
	// HostKey is the server's public key (to pre-trust in tests).
	HostKey ssh.PublicKey
}

// Start listens on addr ("127.0.0.1:0" for a random port). Only the given
// username/password is accepted.
func Start(addr string, dev *Device, user, pass string) (*Server, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
			if c.User() == user && string(p) == pass {
				return nil, nil
			}
			return nil, errors.New("access denied")
		},
	}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{Device: dev, ln: ln, cfg: cfg, HostKey: signer.PublicKey()}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

// Addr is the listening address.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Close stops the listener.
func (s *Server) Close() error {
	err := s.ln.Close()
	s.wg.Wait()
	return err
}

func (s *Server) serve() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(c)
	}
}

func (s *Server) handleConn(c net.Conn) {
	defer c.Close()
	_, chans, reqs, err := ssh.NewServerConn(c, s.cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			_ = nc.Reject(ssh.UnknownChannelType, "only session channels")
			continue
		}
		ch, creqs, err := nc.Accept()
		if err != nil {
			return
		}
		shell := make(chan struct{}, 1)
		go func() {
			for r := range creqs {
				ok := r.Type == "pty-req" || r.Type == "shell" || r.Type == "env" || r.Type == "window-change"
				if r.WantReply {
					_ = r.Reply(ok, nil)
				}
				if r.Type == "shell" {
					shell <- struct{}{}
				}
			}
		}()
		<-shell
		s.cli(ch)
		_ = ch.Close()
	}
}

// cli echoes input byte by byte (like a real device) and runs each line.
func (s *Server) cli(ch ssh.Channel) {
	sess := &session{d: s.Device, mode: modePriv}
	if s.Device.Unprivileged {
		sess.mode = modeExec
	}
	write := func(str string) { _, _ = ch.Write([]byte(str)) }
	write("\r\n" + sess.prompt())
	var line []byte
	buf := make([]byte, 1024)
	lastCR := false
	for {
		n, err := ch.Read(buf)
		if err != nil {
			return
		}
		for _, b := range buf[:n] {
			switch {
			case b == '\n' && lastCR:
				lastCR = false
				continue
			case b == '\r' || b == '\n':
				lastCR = b == '\r'
				out, closeSession := sess.handle(string(line))
				line = line[:0]
				if closeSession {
					write("\r\n")
					return
				}
				write("\r\n")
				if out != "" {
					write(strings.ReplaceAll(out, "\n", "\r\n") + "\r\n")
				}
				write(sess.prompt())
			default:
				lastCR = false
				line = append(line, b)
				_, _ = ch.Write([]byte{b})
			}
		}
	}
}
