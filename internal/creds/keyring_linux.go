//go:build linux

package creds

import (
	"time"

	"golang.org/x/sys/unix"
)

const storeName = "linux kernel keyring"

// keyring stores secrets as "user" keys in the per-UID user keyring. The
// kernel deletes them when the timeout expires. Permissions are restricted
// to the owning UID (possessor + user), so other users cannot read them.
type keyring struct{}

func osStore() Store {
	// Probe once: some containers / seccomp profiles block keyctl.
	if _, err := unix.KeyctlGetKeyringID(unix.KEY_SPEC_USER_KEYRING, true); err != nil {
		return nil
	}
	return keyring{}
}

const perm = 0x3f3f0000 // possessor: all, user: all, group/other: none

func (keyring) Put(k, secret string, ttl time.Duration) error {
	id, err := unix.AddKey("user", k, []byte(secret), unix.KEY_SPEC_USER_KEYRING)
	if err != nil {
		return err
	}
	if err := unix.KeyctlSetperm(id, perm); err != nil {
		return err
	}
	secs := int(ttl.Seconds())
	if secs < 1 {
		secs = 1
	}
	_, err = unix.KeyctlInt(unix.KEYCTL_SET_TIMEOUT, id, secs, 0, 0)
	return err
}

func (keyring) Get(k string) (string, bool) {
	id, err := unix.KeyctlSearch(unix.KEY_SPEC_USER_KEYRING, "user", k, 0)
	if err != nil {
		return "", false
	}
	buf := make([]byte, 4096)
	n, err := unix.KeyctlBuffer(unix.KEYCTL_READ, id, buf, 0)
	if err != nil || n > len(buf) {
		return "", false
	}
	return string(buf[:n]), true
}

func (keyring) Delete(k string) {
	if id, err := unix.KeyctlSearch(unix.KEY_SPEC_USER_KEYRING, "user", k, 0); err == nil {
		_, _ = unix.KeyctlInt(unix.KEYCTL_UNLINK, id, unix.KEY_SPEC_USER_KEYRING, 0, 0)
	}
}
