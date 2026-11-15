package creds

import (
	"fmt"
	"os"
	"testing"
	"time"
)

type fakeStore map[string]string

func (f fakeStore) Get(k string) (string, bool)            { s, ok := f[k]; return s, ok }
func (f fakeStore) Put(k, s string, _ time.Duration) error { f[k] = s; return nil }
func (f fakeStore) Delete(k string)                        { delete(f, k) }

func TestMemoryTTL(t *testing.T) {
	now := time.Unix(1000, 0)
	m := &Manager{ttl: 5 * time.Minute, mem: map[string]memEntry{}, now: func() time.Time { return now }}
	m.Put("admin", "pw")
	now = now.Add(4 * time.Minute)
	if s, ok := m.Get("admin"); !ok || s != "pw" {
		t.Fatal("should be cached at 4m")
	}
	// Get slid the expiry: still valid 4 more minutes later.
	now = now.Add(4 * time.Minute)
	if _, ok := m.Get("admin"); !ok {
		t.Fatal("sliding expiry not applied")
	}
	now = now.Add(6 * time.Minute)
	if _, ok := m.Get("admin"); ok {
		t.Fatal("should have expired")
	}
}

func TestForgetAndStore(t *testing.T) {
	fs := fakeStore{}
	m := &Manager{ttl: time.Minute, store: fs, mem: map[string]memEntry{}, now: time.Now}
	m.Put("admin", "pw")
	if fs["lazynet:admin"] != "pw" {
		t.Fatal("not written to store")
	}
	// A new process (empty memory) finds it in the store.
	m2 := &Manager{ttl: time.Minute, store: fs, mem: map[string]memEntry{}, now: time.Now}
	if s, ok := m2.Get("admin"); !ok || s != "pw" {
		t.Fatal("not read from store")
	}
	m2.Forget("admin")
	if _, ok := fs["lazynet:admin"]; ok {
		t.Fatal("forget must delete from store")
	}
}

func TestZeroTTLNeverTouchesStore(t *testing.T) {
	m := NewManager(0)
	if m.store != nil {
		t.Fatal("ttl 0 must not use the OS store")
	}
	m.Put("admin", "pw")
	if _, ok := m.Get("admin"); !ok {
		t.Fatal("should be remembered for the process")
	}
}

func TestOSKeyring(t *testing.T) {
	s := osStore()
	if s == nil {
		t.Skip("no OS keyring available")
	}
	k := fmt.Sprintf("lazynet-test:%d", os.Getpid())
	if err := s.Put(k, "s3cret", 2*time.Second); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.Get(k); !ok || got != "s3cret" {
		t.Fatalf("get = %q %v", got, ok)
	}
	s.Delete(k)
	if _, ok := s.Get(k); ok {
		t.Fatal("delete failed")
	}
	if err := s.Put(k, "x", 1*time.Second); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1500 * time.Millisecond)
	if _, ok := s.Get(k); ok {
		t.Fatal("kernel did not expire the key")
	}
}
