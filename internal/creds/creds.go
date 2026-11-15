// Package creds remembers device passwords for a short grace period, like
// sudo does. Passwords are never written to disk: on Linux they live in the
// kernel keyring with a kernel-enforced expiry, so the grace period works
// across separate lazynet invocations without any background daemon.
package creds

import (
	"sync"
	"time"
)

// Store is a secret backend with expiring entries.
type Store interface {
	Get(key string) (string, bool)
	// Put stores (or replaces) the secret and (re)sets its expiry.
	Put(key, secret string, ttl time.Duration) error
	Delete(key string)
}

// Manager combines an in-process cache with the OS store.
type Manager struct {
	ttl   time.Duration
	store Store
	mu    sync.Mutex
	mem   map[string]memEntry
	now   func() time.Time
}

type memEntry struct {
	secret  string
	expires time.Time // zero = valid for the life of the process
}

// NewManager returns a manager with the given grace period. ttl <= 0 never
// stores the password outside the process: it is remembered only while this
// lazynet process runs.
func NewManager(ttl time.Duration) *Manager {
	m := &Manager{ttl: ttl, mem: map[string]memEntry{}, now: time.Now}
	if ttl > 0 {
		m.store = osStore()
	}
	return m
}

func key(user string) string { return "lazynet:" + user }

// Get returns a cached password and slides its expiry (like sudo).
func (m *Manager) Get(user string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.mem[user]; ok && (e.expires.IsZero() || m.now().Before(e.expires)) {
		m.touch(user, e.secret)
		return e.secret, true
	}
	delete(m.mem, user)
	if m.store != nil {
		if s, ok := m.store.Get(key(user)); ok {
			m.touch(user, s)
			return s, true
		}
	}
	return "", false
}

// Put remembers a password that just worked.
func (m *Manager) Put(user, secret string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.touch(user, secret)
}

func (m *Manager) touch(user, secret string) {
	e := memEntry{secret: secret}
	if m.ttl > 0 {
		e.expires = m.now().Add(m.ttl)
		if m.store != nil {
			_ = m.store.Put(key(user), secret, m.ttl)
		}
	}
	m.mem[user] = e
}

// Forget drops a password (e.g. after an authentication failure).
func (m *Manager) Forget(user string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.mem, user)
	if m.store != nil {
		m.store.Delete(key(user))
	}
}

// Backend names the OS store in use (for `lazynet doctor`-style output).
func (m *Manager) Backend() string {
	if m.store == nil {
		return "memory (this process only)"
	}
	return storeName
}
