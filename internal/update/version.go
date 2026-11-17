// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

// Package update finds new lazynet releases, installs them the same way
// lazynet was installed, and keeps track of the release notes the user has
// already seen. It never talks to network devices.
package update

import (
	"strconv"
	"strings"
)

// Version is a semantic version: MAJOR.MINOR.PATCH with an optional
// pre-release suffix.
type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

// ParseVersion accepts "1.2.3", "v1.2.3" and "1.2.3-rc.1".
func ParseVersion(s string) (Version, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	var v Version
	core, pre, _ := strings.Cut(s, "-")
	v.Pre = pre
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" || (len(p) > 1 && p[0] == '0') {
			return Version{}, false
		}
		switch i {
		case 0:
			v.Major = n
		case 1:
			v.Minor = n
		case 2:
			v.Patch = n
		}
	}
	return v, true
}

// IsRelease reports whether s is a published release version (X.Y.Z, no
// suffix). Builds from a git checkout ("dev", "v0.1.0-3-gabc1234") and
// pseudo-versions from `go install …@main` are not releases: they are never
// offered updates and never show release notes.
func IsRelease(s string) bool {
	v, ok := ParseVersion(s)
	return ok && v.Pre == ""
}

// Compare returns -1, 0 or +1 when a is older than, equal to or newer than b.
// A version that doesn't parse is older than any version that does.
func Compare(a, b string) int {
	va, oka := ParseVersion(a)
	vb, okb := ParseVersion(b)
	switch {
	case !oka && !okb:
		return 0
	case !oka:
		return -1
	case !okb:
		return 1
	}
	for _, d := range [3]int{va.Major - vb.Major, va.Minor - vb.Minor, va.Patch - vb.Patch} {
		if d != 0 {
			return sign(d)
		}
	}
	switch {
	case va.Pre == vb.Pre:
		return 0
	case va.Pre == "": // a release is newer than its pre-releases
		return 1
	case vb.Pre == "":
		return -1
	}
	return sign(strings.Compare(va.Pre, vb.Pre))
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// String renders the version without a "v" prefix.
func (v Version) String() string {
	s := strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}
