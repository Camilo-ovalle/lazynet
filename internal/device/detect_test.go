// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package device

import (
	"os"
	"strings"
	"testing"
)

// Detection runs `show version | include Software`; this filters the C9200L
// fixture the same way the device would.
func TestDetectCatalyst9200L(t *testing.T) {
	b, err := os.ReadFile("../parse/testdata/catalyst_c9200l-48p-4x_17.12.04_show_version.txt")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.Contains(l, "Software") {
			lines = append(lines, l)
		}
	}
	p, err := DetectFromVersion(strings.Join(lines, "\n"))
	if err != nil || p != Catalyst {
		t.Fatalf("got %q, %v", p, err)
	}
}
