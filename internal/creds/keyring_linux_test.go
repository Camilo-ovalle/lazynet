//go:build linux

package creds

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"golang.org/x/sys/unix"
)

// TestKeyringWithoutPossession reproduces CI runners, systemd services and
// some SSH sessions, where the user keyring is not linked into the session
// keyring. The process then does not possess keys it adds to the user
// keyring. A key added there directly could not get its permissions or its
// timeout, and the secret was left behind with no expiry.
//
// The test runs in a child process because joining a new session keyring
// cannot be undone.
func TestKeyringWithoutPossession(t *testing.T) {
	if os.Getenv("LAZYNET_KEYRING_CHILD") == "1" {
		if _, err := unix.KeyctlJoinSessionKeyring(fmt.Sprintf("lazynet-test-%d", os.Getpid())); err != nil {
			t.Skipf("cannot join a new session keyring: %v", err)
		}
		s := osStore()
		if s == nil {
			t.Skip("no usable OS keyring")
		}
		checkStore(t, s)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestKeyringWithoutPossession$", "-test.v")
	cmd.Env = append(os.Environ(), "LAZYNET_KEYRING_CHILD=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child failed: %v\n%s", err, out)
	}
}
