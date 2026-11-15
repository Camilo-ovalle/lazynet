// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package app wires configuration, inventory, credentials, transport and
// drivers together. Both the CLI and the TUI drive devices through it, so
// safety rules (confirmation, audit, read-only guard) live in one place.
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Camilo-ovalle/lazynet/internal/audit"
	"github.com/Camilo-ovalle/lazynet/internal/backup"
	"github.com/Camilo-ovalle/lazynet/internal/config"
	"github.com/Camilo-ovalle/lazynet/internal/creds"
	"github.com/Camilo-ovalle/lazynet/internal/device"
	"github.com/Camilo-ovalle/lazynet/internal/inventory"
	"github.com/Camilo-ovalle/lazynet/internal/transport"
)

// Prompter asks the human for things mid-operation. The CLI implements it
// with terminal prompts, the TUI with modal dialogs.
type Prompter interface {
	Username(target string) (string, error)
	// Password is called with retry=true after an authentication failure.
	Password(user, target string, retry bool) (string, error)
	TrustHostKey(addr, keyType, fingerprint string) (bool, error)
}

// ErrHostKeyRejected is returned when the user does not trust a new key.
var ErrHostKeyRejected = errors.New("host key not trusted: connection aborted")

// App is the shared application context.
type App struct {
	Paths config.Paths
	Cfg   config.Config
	Inv   *inventory.Inventory
	Creds *creds.Manager
	Audit *audit.Log
	Demo  bool

	promptMu sync.Mutex
	detectMu sync.Mutex
	backupMu sync.Mutex
	backups  *backup.Store
}

// Load builds the App from the default paths.
func Load() (*App, error) {
	p, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}
	return LoadFrom(p)
}

// LoadFrom builds the App from explicit paths.
func LoadFrom(p config.Paths) (*App, error) {
	if err := p.Ensure(); err != nil {
		return nil, err
	}
	cfg, err := config.Load(p.Config)
	if err != nil {
		return nil, err
	}
	inv, err := inventory.Load(p.Inventory)
	if err != nil {
		return nil, err
	}
	return &App{Paths: p, Cfg: cfg, Inv: inv, Creds: creds.NewManager(cfg.CredentialTTL.D()), Audit: audit.New(p.AuditLog)}, nil
}

// Session is an open connection with its detected driver.
type Session struct {
	Device   inventory.Device
	User     string
	Conn     *transport.Conn
	Platform device.Platform
	Driver   device.Driver
}

// Close ends the SSH session.
func (s *Session) Close() error { return s.Conn.Close() }

func (a *App) username(d inventory.Device, pr Prompter) (string, error) {
	if d.Username != "" {
		return d.Username, nil
	}
	if a.Cfg.DefaultUsername != "" {
		return a.Cfg.DefaultUsername, nil
	}
	return pr.Username(d.Label())
}

// password returns a cached password or prompts once. Prompts are
// serialised so concurrent group operations ask only once per user.
func (a *App) password(user, target string, pr Prompter, retry bool) (string, error) {
	a.promptMu.Lock()
	defer a.promptMu.Unlock()
	if !retry {
		if pw, ok := a.Creds.Get(user); ok {
			return pw, nil
		}
	}
	pw, err := pr.Password(user, target, retry)
	if err != nil {
		return "", err
	}
	a.Creds.Put(user, pw)
	return pw, nil
}

// Connect opens a session: credentials (with the sudo-like grace period),
// host key trust-on-first-use, privilege check and platform detection.
func (a *App) Connect(d inventory.Device, pr Prompter) (*Session, error) {
	a.promptMu.Lock()
	user, err := a.username(d, pr)
	a.promptMu.Unlock()
	if err != nil {
		return nil, err
	}
	opts := transport.Options{
		Host: d.Host, Port: d.SSHPort(), Username: user,
		KnownHostsFile: a.Paths.KnownHosts,
		DialTimeout:    a.Cfg.SSHTimeout.D(), CommandTimeout: a.Cfg.CommandTimeout.D(),
		LegacySSH: a.Cfg.LegacySSH,
	}
	// Like ssh: verify the host (and that it is reachable) before asking for
	// a password, so a password is never typed for an untrusted device.
	if _, cached := a.Creds.Get(user); !cached {
		if err := a.verifyHost(opts, pr); err != nil {
			return nil, err
		}
	}
	pw, err := a.password(user, d.Label(), pr, false)
	if err != nil {
		return nil, err
	}
	var conn *transport.Conn
	authFails := 0
	for conn == nil {
		opts.Password = pw
		c, err := transport.Dial(opts)
		var uk *transport.UnknownHostKeyError
		switch {
		case err == nil:
			conn = c
		case errors.As(err, &uk):
			if err := a.trust(uk, pr); err != nil {
				return nil, err
			}
		case errors.Is(err, transport.ErrAuth):
			a.Creds.Forget(user)
			if authFails++; authFails >= 3 {
				return nil, err
			}
			if pw, err = a.password(user, d.Label(), pr, true); err != nil {
				return nil, err
			}
		default:
			return nil, err
		}
	}
	p, err := device.ParsePlatform(d.Platform)
	if err == nil && p == device.Unknown {
		p, err = device.Detect(conn)
	}
	if err == nil {
		err = device.Setup(p, conn)
	}
	var drv device.Driver
	if err == nil {
		drv, err = device.New(p, conn)
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	a.rememberPlatform(d.Label(), p)
	return &Session{Device: d, User: user, Conn: conn, Platform: p, Driver: drv}, nil
}

func (a *App) verifyHost(o transport.Options, pr Prompter) error {
	err := transport.CheckHostKey(o)
	var uk *transport.UnknownHostKeyError
	if errors.As(err, &uk) {
		return a.trust(uk, pr)
	}
	return err
}

// trust asks the user about an unknown key and records it if accepted.
func (a *App) trust(uk *transport.UnknownHostKeyError, pr Prompter) error {
	a.promptMu.Lock()
	defer a.promptMu.Unlock()
	// A concurrent connection (group operation) may have trusted it already.
	if transport.IsKnown(a.Paths.KnownHosts, uk) {
		return nil
	}
	ok, err := pr.TrustHostKey(uk.Addr, uk.Key.Type(), transport.Fingerprint(uk.Key))
	if err != nil {
		return err
	}
	if !ok {
		return ErrHostKeyRejected
	}
	return transport.TrustHostKey(a.Paths.KnownHosts, uk)
}

// --- detection cache (shown as badges in the TUI device list) ---

// DetectedPlatforms returns label -> platform detected on previous connects.
func (a *App) DetectedPlatforms() map[string]device.Platform {
	a.detectMu.Lock()
	defer a.detectMu.Unlock()
	m := map[string]device.Platform{}
	if b, err := os.ReadFile(a.Paths.Detect); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	return m
}

func (a *App) rememberPlatform(label string, p device.Platform) {
	m := a.DetectedPlatforms()
	if m[label] == p {
		return
	}
	a.detectMu.Lock()
	defer a.detectMu.Unlock()
	m[label] = p
	if b, err := json.MarshalIndent(m, "", "  "); err == nil {
		_ = os.WriteFile(a.Paths.Detect, b, 0o600)
	}
}

// --- backups ---

// Backups opens (once) the git-backed backup store.
func (a *App) Backups() (*backup.Store, error) {
	a.backupMu.Lock()
	defer a.backupMu.Unlock()
	if a.backups == nil {
		s, err := backup.Open(a.Paths.Backups)
		if err != nil {
			return nil, err
		}
		a.backups = s
	}
	return a.backups, nil
}

// Backup fetches running-config and stores it.
func (a *App) Backup(s *Session) (backup.Result, error) {
	cfg, err := s.Driver.RunningConfig()
	if err != nil {
		return backup.Result{}, err
	}
	st, err := a.Backups()
	if err != nil {
		return backup.Result{}, err
	}
	return st.Save(s.Device.Label(), s.Device.Host, cfg, time.Now())
}

// --- guarded changes (always audited) ---

// PlanVlan validates a VLAN change using show commands only.
func (a *App) PlanVlan(s *Session, port string, vlan int) (*device.VlanPlan, error) {
	return device.PlanVlanChange(s.Conn, s.Platform, port, vlan)
}

func (a *App) record(s *Session, action string, cmds []string) audit.Record {
	return audit.Record{DeviceUser: s.User, Device: s.Device.Label(), Host: s.Device.Host, Action: action, Commands: cmds}
}

// DeclineVlan audits a change the user chose not to apply.
func (a *App) DeclineVlan(d inventory.Device, user string, plan *device.VlanPlan) {
	r := audit.Record{DeviceUser: user, Device: d.Label(), Host: d.Host, Action: "vlan-change", Commands: plan.Change.Display()}
	r.Port, r.FromVlan, r.ToVlan, r.Result = plan.Port, plan.CurrentVlan, plan.TargetVlan, "declined"
	_ = a.Audit.Write(r)
}

// ApplyVlan sends a confirmed plan, verifies it and writes the audit record.
// The caller must have called plan.Change.Confirm with the displayed lines.
func (a *App) ApplyVlan(s *Session, plan *device.VlanPlan) error {
	r := a.record(s, "vlan-change", plan.Change.Display())
	r.Port, r.FromVlan, r.ToVlan = plan.Port, plan.CurrentVlan, plan.TargetVlan
	_, err := device.ApplyVlanChange(s.Conn, plan)
	r.Result = "applied"
	if err != nil {
		r.Result, r.Error = "failed", err.Error()
	}
	if werr := a.Audit.Write(r); werr != nil && err == nil {
		return fmt.Errorf("change applied but audit log failed: %w", werr)
	}
	return err
}

// SaveConfig sends a confirmed "write memory" and audits it.
func (a *App) SaveConfig(s *Session, cr *transport.ChangeRequest) error {
	r := a.record(s, "save-config", cr.Display())
	_, err := s.Conn.Apply(cr)
	r.Result, r.Saved = "applied", err == nil
	if err != nil {
		r.Result, r.Error = "failed", err.Error()
	}
	_ = a.Audit.Write(r)
	return err
}

// Result of a per-device group operation.
type Result struct {
	Device inventory.Device
	Value  any
	Err    error
}

// ForEach runs fn for every device with at most Cfg.Workers in parallel and
// returns results in input order.
func (a *App) ForEach(devs []inventory.Device, fn func(inventory.Device) (any, error)) []Result {
	res := make([]Result, len(devs))
	sem := make(chan struct{}, max(1, a.Cfg.Workers))
	var wg sync.WaitGroup
	for i, d := range devs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			v, err := fn(d)
			res[i] = Result{Device: d, Value: v, Err: err}
		}()
	}
	wg.Wait()
	return res
}
