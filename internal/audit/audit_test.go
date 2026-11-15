// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "audit.log")
	l := New(p)
	for i := 0; i < 2; i++ {
		if err := l.Write(Record{Device: "sw1", Action: "vlan-change", Commands: []string{"x"}, Result: "applied"}); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var r Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil || r.LocalUser == "" || r.Time.IsZero() {
			t.Fatalf("bad record %s: %v", sc.Text(), err)
		}
		n++
	}
	if n != 2 {
		t.Fatalf("got %d records", n)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perm %v", st.Mode().Perm())
	}
}
