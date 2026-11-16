// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/scrapli/scrapligo/driver/network"
	"github.com/scrapli/scrapligo/driver/opoptions"
	"github.com/scrapli/scrapligo/driver/options"
	"github.com/scrapli/scrapligo/platform"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Errors returned by Dial, classified so the UI can react (re-prompt for a
// password, ask to trust a host key, ...).
var (
	ErrAuth           = errors.New("authentication failed")
	ErrNotPrivileged  = errors.New("logged in at user EXEC (>): lazynet needs privilege 15 (#); ask your admin for 'privilege 15' on this account")
	ErrUnreachable    = errors.New("device unreachable")
	ErrHostKeyChanged = errors.New("HOST KEY CHANGED: the device presented a different SSH key than the one in known_hosts. This may be a man-in-the-middle attack or a replaced device. Remove the old line from known_hosts only if you are sure")
)

// UnknownHostKeyError means the device's key is not in known_hosts yet.
type UnknownHostKeyError struct {
	Addr string
	Key  ssh.PublicKey
}

func (e *UnknownHostKeyError) Error() string {
	return fmt.Sprintf("unknown SSH host key for %s (%s %s)", e.Addr, e.Key.Type(), Fingerprint(e.Key))
}

// Fingerprint renders a key the same way OpenSSH does.
func Fingerprint(k ssh.PublicKey) string { return ssh.FingerprintSHA256(k) }

// Options configure a connection.
type Options struct {
	Host           string
	Port           int
	Username       string
	Password       string
	KnownHostsFile string
	DialTimeout    time.Duration
	CommandTimeout time.Duration
	LegacySSH      bool
	// Dialect describes the CLIs this session may meet (platform.Dialect).
	Dialect Dialect
	// ChannelLog, if set, receives the raw session transcript (debugging).
	ChannelLog io.Writer
}

// Conn is an open SSH session to a device, at privilege 15 ("#") or at a
// platform's read-only operational prompt.
type Conn struct {
	mu       sync.Mutex
	d        *network.Driver
	prompt   string
	addr     string
	session  []string // exact session commands Show accepts besides "show …"
	readOnly bool     // opened at an operational '>' prompt: Apply is refused
}

// ErrRejected means the device answered a command with one of the dialect's
// failure markers (e.g. "% Invalid input detected").
var ErrRejected = errors.New("device rejected command")

// ErrChangesUnsupported is returned by Apply on read-only sessions (opened at
// an operational '>' prompt, such as PAN-OS).
var ErrChangesUnsupported = errors.New("configuration changes are not supported on this platform")

// Dial opens the session and verifies the login landed where lazynet can work:
// privilege 15 ("#"), or a '>' prompt the dialect accepts as an operational
// (read-only) mode. Any other '>' prompt is refused with ErrNotPrivileged.
func Dial(o Options) (*Conn, error) {
	if err := o.Dialect.validate(); err != nil {
		return nil, err
	}
	def, err := o.Dialect.definition()
	if err != nil {
		return nil, fmt.Errorf("platform definition: %w", err)
	}
	if o.Port == 0 {
		o.Port = 22
	}
	addr := net.JoinHostPort(o.Host, fmt.Sprint(o.Port))
	if err := ensureFile(o.KnownHostsFile); err != nil {
		return nil, err
	}
	opts := []util.Option{
		options.WithTransportType(transport.StandardTransport),
		options.WithPort(o.Port),
		options.WithAuthUsername(o.Username),
		options.WithAuthPassword(o.Password),
		options.WithSSHKnownHostsFile(o.KnownHostsFile),
		options.WithTimeoutSocket(o.DialTimeout),
		options.WithTimeoutOps(o.CommandTimeout),
		options.WithTermWidth(511),
	}
	if o.LegacySSH {
		a := legacyAlgorithms()
		opts = append(opts,
			options.WithStandardTransportExtraCiphers(a.Ciphers),
			options.WithStandardTransportExtraKexs(a.KeyExchanges),
			options.WithStandardTransportHostKeyAlgorithms(a.HostKeys),
		)
	}
	if o.ChannelLog != nil {
		opts = append(opts, options.WithChannelLog(o.ChannelLog))
	}
	p, err := platform.NewPlatform(def, o.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("platform definition: %w", err)
	}
	d, err := p.GetNetworkDriver()
	if err != nil {
		return nil, err
	}
	if err := d.Open(); err != nil {
		return nil, classify(addr, err)
	}
	prompt, err := d.GetPrompt()
	if err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("reading prompt: %w", err)
	}
	prompt = strings.TrimSpace(prompt)
	readOnly := false
	switch {
	case strings.HasSuffix(prompt, ">") && o.Dialect.OperationalPrompt != nil && o.Dialect.OperationalPrompt(prompt):
		// A platform's operational mode is where lazynet works: never try
		// to "escalate" from it, and never send changes on it.
		d.CurrentPriv, d.DefaultDesiredPriv = "exec", "exec"
		readOnly = true
	case strings.HasSuffix(prompt, ">"):
		_ = d.Channel.Close()
		return nil, ErrNotPrivileged
	default:
		d.CurrentPriv = "privilege-exec"
	}
	return &Conn{d: d, prompt: prompt, addr: addr, session: slices.Clone(o.Dialect.SessionCommands), readOnly: readOnly}, nil
}

// legacyAlgorithms is every modern algorithm (preferred) followed by the
// legacy ones old IOS images need. The order must be deterministic so the
// same host key type is negotiated on every connection.
func legacyAlgorithms() ssh.Algorithms {
	sup, insec := ssh.SupportedAlgorithms(), ssh.InsecureAlgorithms()
	return ssh.Algorithms{
		Ciphers: append(sup.Ciphers, pick(insec.Ciphers, "aes128-cbc", "aes256-cbc", "3des-cbc")...),
		KeyExchanges: append(sup.KeyExchanges, pick(insec.KeyExchanges,
			"diffie-hellman-group-exchange-sha1", "diffie-hellman-group14-sha1", "diffie-hellman-group1-sha1")...),
		HostKeys: append(sup.HostKeys, pick(insec.HostKeys, ssh.KeyAlgoRSA)...),
	}
}

var errProbeDone = errors.New("probe done")

// CheckHostKey performs only the SSH key exchange (no authentication is
// attempted, so nothing shows up as a failed login on the device) and
// verifies the server key against known_hosts. Call it before asking the user
// for a password: it returns *UnknownHostKeyError, ErrHostKeyChanged,
// ErrUnreachable or nil.
func CheckHostKey(o Options) error {
	if o.Port == 0 {
		o.Port = 22
	}
	addr := net.JoinHostPort(o.Host, fmt.Sprint(o.Port))
	if err := ensureFile(o.KnownHostsFile); err != nil {
		return err
	}
	kh, err := knownhosts.New(o.KnownHostsFile)
	if err != nil {
		return err
	}
	var verdict error
	cfg := &ssh.ClientConfig{
		User:    o.Username,
		Timeout: o.DialTimeout,
		HostKeyCallback: func(host string, remote net.Addr, key ssh.PublicKey) error {
			verdict = kh(host, remote, key)
			var ke *knownhosts.KeyError
			switch {
			case verdict == nil:
			case errors.As(verdict, &ke) && len(ke.Want) > 0:
				verdict = fmt.Errorf("%s: %w", addr, ErrHostKeyChanged)
			default:
				verdict = &UnknownHostKeyError{Addr: addr, Key: key}
			}
			return errProbeDone // abort before authentication
		},
	}
	if o.LegacySSH {
		a := legacyAlgorithms()
		cfg.Ciphers, cfg.KeyExchanges, cfg.HostKeyAlgorithms = a.Ciphers, a.KeyExchanges, a.HostKeys
	}
	c, err := net.DialTimeout("tcp", addr, o.DialTimeout)
	if err != nil {
		return fmt.Errorf("%s: %w (%v)", addr, ErrUnreachable, err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(o.DialTimeout))
	_, _, _, err = ssh.NewClientConn(c, addr, cfg)
	if errors.Is(err, errProbeDone) {
		return verdict
	}
	if err == nil {
		return errors.New("unexpected: SSH handshake completed without host key check")
	}
	return classify(addr, err)
}

func pick(have []string, want ...string) []string {
	var out []string
	for _, w := range want {
		if slices.Contains(have, w) {
			out = append(out, w)
		}
	}
	return out
}

func classify(addr string, err error) error {
	var hk *transport.HostKeyError
	if errors.As(err, &hk) {
		var ke *knownhosts.KeyError
		if errors.As(hk.Err, &ke) && len(ke.Want) > 0 {
			return fmt.Errorf("%s: %w", addr, ErrHostKeyChanged)
		}
		return &UnknownHostKeyError{Addr: addr, Key: hk.HostKey}
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unable to authenticate"), strings.Contains(msg, "no supported methods remain"):
		return fmt.Errorf("%s: %w", addr, ErrAuth)
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "i/o timeout"),
		strings.Contains(msg, "no route to host"), strings.Contains(msg, "network is unreachable"),
		strings.Contains(msg, "no such host"):
		return fmt.Errorf("%s: %w (%v)", addr, ErrUnreachable, err)
	}
	return fmt.Errorf("%s: %w", addr, err)
}

// TrustHostKey appends the key to known_hosts (trust on first use).
func TrustHostKey(knownHostsFile string, e *UnknownHostKeyError) error {
	if err := ensureFile(knownHostsFile); err != nil {
		return err
	}
	f, err := os.OpenFile(knownHostsFile, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line := knownhosts.Line([]string{knownhosts.Normalize(e.Addr)}, e.Key)
	_, err = fmt.Fprintln(f, line)
	return err
}

// IsKnown reports whether known_hosts already accepts this exact key.
func IsKnown(knownHostsFile string, e *UnknownHostKeyError) bool {
	kh, err := knownhosts.New(knownHostsFile)
	if err != nil {
		return false
	}
	addr, err := net.ResolveTCPAddr("tcp", e.Addr)
	if err != nil {
		return false
	}
	return kh(e.Addr, addr, e.Key) == nil
}

func ensureFile(path string) error {
	if path == "" {
		return errors.New("known_hosts path not configured")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return err
	}
	return f.Close()
}

// Prompt returns the device prompt seen at login (e.g. "core-sw1#").
func (c *Conn) Prompt() string { return c.prompt }

// ReadOnly reports whether the session was opened at an operational '>'
// prompt, where Apply is always refused.
func (c *Conn) ReadOnly() bool { return c.readOnly }

// Hostname is the host part of the prompt: without the trailing '#' or '>',
// a "user@" prefix, a "(state)" suffix or an ASA "/context".
func (c *Conn) Hostname() string {
	h := strings.TrimRight(c.prompt, "#> ")
	if i := strings.LastIndexByte(h, '@'); i >= 0 {
		h = h[i+1:]
	}
	if i := strings.IndexAny(h, "(/"); i > 0 {
		h = h[:i]
	}
	return h
}

// Addr returns host:port.
func (c *Conn) Addr() string { return c.addr }

// Show runs a read-only command. Anything that is not a 'show' or one of the
// dialect's session settings is rejected before it reaches the network.
func (c *Conn) Show(cmd string) (string, error) {
	if err := CheckReadOnly(cmd, c.session...); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	r, err := c.d.SendCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("%s: %w", cmd, err)
	}
	if r.Failed != nil {
		return r.Result, fmt.Errorf("%s: %w: %s", cmd, ErrRejected, firstErrorLine(r.Result))
	}
	return r.Result, nil
}

// Apply sends a confirmed ChangeRequest and returns the device output.
func (c *Conn) Apply(cr *ChangeRequest) (string, error) {
	if cr == nil || !cr.Confirmed() {
		return "", ErrNotConfirmed
	}
	if c.readOnly {
		return "", ErrChangesUnsupported
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	switch cr.kind {
	case ExecCommand:
		r, err := c.d.SendCommand(cr.lines[0])
		if err != nil {
			return "", err
		}
		if r.Failed != nil {
			return r.Result, fmt.Errorf("device rejected %q: %s", cr.lines[0], firstErrorLine(r.Result))
		}
		return r.Result, nil
	default:
		mr, err := c.d.SendConfigs(cr.lines, opoptions.WithStopOnFailed())
		// Always leave configuration mode, even after a failure.
		if perr := c.d.AcquirePriv("privilege-exec"); perr != nil && err == nil {
			err = perr
		}
		if err != nil {
			return "", err
		}
		var out strings.Builder
		for _, r := range mr.Responses {
			out.WriteString(r.Result)
			if r.Failed != nil {
				return out.String(), fmt.Errorf("device rejected %q: %s", r.Input, firstErrorLine(r.Result))
			}
		}
		return out.String(), nil
	}
}

var errLine = regexp.MustCompile(`(?m)^.*(%|ERROR).*$`)

func firstErrorLine(out string) string {
	if m := errLine.FindString(out); m != "" {
		return strings.TrimSpace(m)
	}
	return strings.TrimSpace(out)
}

// Close ends the session politely.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.d.Channel.Write([]byte("exit"), false)
	_ = c.d.Channel.WriteReturn()
	return c.d.Channel.Close()
}
