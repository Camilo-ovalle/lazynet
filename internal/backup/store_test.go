package backup

import (
	"strings"
	"testing"
	"time"
)

func TestSaveListDiff(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	cfg1 := "Building configuration...\n\nCurrent configuration : 100 bytes\nhostname sw1\ninterface Gi1/0/1\n switchport access vlan 10\nend\n"
	r1, err := s.Save("core-sw1", "10.0.0.1", cfg1, t0)
	if err != nil || !r1.Changed {
		t.Fatalf("first save: %+v %v", r1, err)
	}
	// Same config with a different "Current configuration" size → no new commit.
	same := strings.Replace(cfg1, "100 bytes", "104 bytes", 1)
	r2, err := s.Save("core-sw1", "10.0.0.1", same, t0.Add(time.Hour))
	if err != nil || r2.Changed || r2.Hash != r1.Hash {
		t.Fatalf("unchanged save should not commit: %+v %v", r2, err)
	}
	cfg3 := strings.Replace(cfg1, "vlan 10", "vlan 110", 1)
	if _, err := s.Save("core-sw1", "10.0.0.1", cfg3, t0.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save("other/sw", "10.0.0.2", "hostname other\n", t0); err != nil {
		t.Fatal(err)
	}
	list, err := s.List("core-sw1")
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %v %v", list, err)
	}
	if !list[0].Time.After(list[1].Time) {
		t.Fatal("list must be newest first")
	}
	d, err := s.Diff("core-sw1", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d, "- switchport access vlan 10") || !strings.Contains(d, "+ switchport access vlan 110") {
		t.Fatalf("diff:\n%s", d)
	}
	if strings.Contains(d, "other") {
		t.Fatal("diff leaked another device")
	}
	c, err := s.Content("core-sw1", list[1].Hash)
	if err != nil || strings.Contains(c, "Building configuration") {
		t.Fatalf("content: %q %v", c, err)
	}
	if _, err := s.Diff("other/sw", "", ""); err == nil {
		t.Fatal("single backup diff should error")
	}
	// Re-open existing repo.
	if _, err := Open(s.Dir()); err != nil {
		t.Fatal(err)
	}
	if DeviceDir("../etc/passwd") != "etc_passwd" {
		t.Fatalf("sanitize: %q", DeviceDir("../etc/passwd"))
	}
}
