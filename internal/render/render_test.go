// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestReleaseNotes(t *testing.T) {
	md := "### Added\n\n\n- `lazynet update` installs the latest release, the same way\n  lazynet was installed.\n- Short.\n\nA closing paragraph that is long enough to be wrapped somewhere.\n\n"
	got := ReleaseNotes(md, 40)
	var plain []string
	for _, l := range got {
		plain = append(plain, ansi.Strip(l))
		if w := ansi.StringWidth(l); w > 40 {
			t.Errorf("line wider than 40 cells (%d): %q", w, ansi.Strip(l))
		}
	}
	want := []string{
		"Added",
		"",
		"  • lazynet update installs the latest",
		"    release, the same way lazynet was",
		"    installed.",
		"  • Short.",
		"",
		"A closing paragraph that is long enough",
		"to be wrapped somewhere.",
	}
	if strings.Join(plain, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(plain, "\n"), strings.Join(want, "\n"))
	}
	if strings.Join(ReleaseNotes("- odd `backtick", 40), "") == "" {
		t.Fatal("unbalanced backticks must still render")
	}
}
