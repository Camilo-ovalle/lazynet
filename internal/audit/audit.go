// Package audit appends a record of every configuration change to a local
// JSON-lines file (one object per line, easy to grep or ship to a SIEM).
package audit

import (
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"time"
)

// Record is one audited change attempt.
type Record struct {
	Time       time.Time `json:"time"`
	LocalUser  string    `json:"local_user"`
	DeviceUser string    `json:"device_user"`
	Device     string    `json:"device"`
	Host       string    `json:"host"`
	Action     string    `json:"action"` // vlan-change | save-config
	Port       string    `json:"port,omitempty"`
	FromVlan   int       `json:"from_vlan,omitempty"`
	ToVlan     int       `json:"to_vlan,omitempty"`
	Commands   []string  `json:"commands"`
	Result     string    `json:"result"` // applied | declined | failed
	Error      string    `json:"error,omitempty"`
	Saved      bool      `json:"saved,omitempty"`
}

// Log writes records to a file.
type Log struct {
	path string
	mu   sync.Mutex
}

// New returns a log writing to path.
func New(path string) *Log { return &Log{path: path} }

// Path returns the file path.
func (l *Log) Path() string { return l.path }

// Write appends r, filling Time and LocalUser if empty.
func (l *Log) Write(r Record) error {
	if r.Time.IsZero() {
		r.Time = time.Now()
	}
	if r.LocalUser == "" {
		if u, err := user.Current(); err == nil {
			r.LocalUser = u.Username
		}
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}
