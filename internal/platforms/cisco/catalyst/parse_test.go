// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package catalyst

import (
	"os"
	"testing"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/platform/platformtest"
)

func TestIOSXEVersion(t *testing.T) {
	v := Version(platformtest.Device("catalyst", "lab1").Exec("show version"))
	want := parse.Version{Hostname: "lab1", OS: "IOS-XE", Version: "17.09.04a", Model: "C9300-48P", Serial: "FCW2318L0AB", MAC: "706d.15a2.3b80", Uptime: "12 weeks, 3 days, 4 hours, 51 minutes"}
	if v != want {
		t.Fatalf("got  %+v\nwant %+v", v, want)
	}
}

// TestIOSXECatalyst9200L covers the switches lazynet targets in production:
// Catalyst C9200L-48P-4X on IOS XE 17.12.04 (ARM64, CAT9K_LITE_IOSXE image).
// The fixture is synthetic, built from the documented format of that model and
// release; replace it with a sanitized real capture when one is available.
func TestIOSXECatalyst9200L(t *testing.T) {
	v := Version(c9200l(t))
	want := parse.Version{Hostname: "acc-sw-01", OS: "IOS-XE", Version: "17.12.04", Model: "C9200L-48P-4X", Serial: "JAE25130ABC", MAC: "3c13.cc4a.1b00", Uptime: "21 weeks, 6 days, 2 hours, 14 minutes"}
	if v != want {
		t.Fatalf("got  %+v\nwant %+v", v, want)
	}
}

func c9200l(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/catalyst_c9200l-48p-4x_17.12.04_show_version.txt")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const classicIOS = `Cisco IOS Software, C2960X Software (C2960X-UNIVERSALK9-M), Version 15.2(7)E8, RELEASE SOFTWARE (fc1)
Technical Support: http://www.cisco.com/techsupport
ROM: Bootstrap program is C2960X boot loader
access-sw2 uptime is 1 year, 2 weeks, 3 days, 5 hours, 1 minute
System image file is "flash:c2960x-universalk9-mz.152-7.E8.bin"
cisco WS-C2960X-48FPD-L (APM86XXX) processor (revision D0) with 524288K bytes of memory.
Processor board ID FOC1951X2AB
Base ethernet MAC Address       : 70:6D:15:11:22:00
Model number                    : WS-C2960X-48FPD-L
System serial number            : FOC1951X2AB
`

func TestClassicIOSVersion(t *testing.T) {
	v := Version(classicIOS)
	if v.OS != "IOS" || v.Version != "15.2(7)E8" || v.Model != "WS-C2960X-48FPD-L" || v.Serial != "FOC1951X2AB" || v.MAC != "706d.1511.2200" || v.Hostname != "access-sw2" {
		t.Fatalf("got %+v", v)
	}
}
