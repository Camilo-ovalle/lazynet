//go:build linux

package creds

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

const storeName = "linux kernel keyring"

// keyring stores secrets as "user" keys in the per-UID user keyring, so they
// are shared by separate lazynet runs of the same user. The kernel deletes
// them when the timeout expires. Permissions are restricted to the owning
// UID (possessor + user), so other users cannot read them.
type keyring struct{}

// osStore returns the kernel keyring if it is actually usable. A full
// put/get/delete round trip is required, not just the presence of a user
// keyring: containers and seccomp profiles can block keyctl, and a store
// that fails halfway must never be used.
func osStore() Store {
	k := fmt.Sprintf("lazynet:probe:%d", os.Getpid())
	s := keyring{}
	if err := s.Put(k, "probe", time.Second); err != nil {
		return nil
	}
	defer s.Delete(k)
	if v, ok := s.Get(k); !ok || v != "probe" {
		return nil
	}
	return s
}

const perm = 0x3f3f0000 // possessor: all, user: all, group/other: none

// Put stores the secret with an expiry.
//
// The key is built in the process keyring and only then linked into the user
// keyring. A process always possesses its own process keyring, so setting
// permissions and the timeout is allowed. A key added directly to the user
// keyring is not possessed when the user keyring is not linked into the
// session keyring (systemd services, CI runners, some SSH or cron sessions).
// In that case setperm and the timeout fail with EACCES, and the secret would
// be left behind with no expiry.
//
// If any step fails, the key is revoked, so a secret is never left in the
// keyring without its timeout.
func (keyring) Put(k, secret string, ttl time.Duration) error {
	id, err := unix.AddKey("user", k, []byte(secret), unix.KEY_SPEC_PROCESS_KEYRING)
	if err != nil {
		return err
	}
	defer unix.KeyctlInt(unix.KEYCTL_UNLINK, id, unix.KEY_SPEC_PROCESS_KEYRING, 0, 0)
	fail := func(err error) error {
		_, _ = unix.KeyctlInt(unix.KEYCTL_REVOKE, id, 0, 0, 0)
		return err
	}
	if err := unix.KeyctlSetperm(id, perm); err != nil {
		return fail(err)
	}
	secs := int(ttl.Seconds())
	if secs < 1 {
		secs = 1
	}
	if _, err := unix.KeyctlInt(unix.KEYCTL_SET_TIMEOUT, id, secs, 0, 0); err != nil {
		return fail(err)
	}
	// Linking replaces any older key with the same description.
	if _, err := unix.KeyctlInt(unix.KEYCTL_LINK, id, unix.KEY_SPEC_USER_KEYRING, 0, 0); err != nil {
		return fail(err)
	}
	return nil
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
