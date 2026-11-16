// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/Camilo-ovalle/lazynet/internal/config"
	"github.com/Camilo-ovalle/lazynet/internal/creds"
	"github.com/Camilo-ovalle/lazynet/internal/inventory"
	"github.com/Camilo-ovalle/lazynet/internal/mockssh"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"golang.org/x/crypto/ssh/knownhosts"
)

// DemoUser / DemoPassword are the credentials of the emulated devices.
const (
	DemoUser     = "demo"
	DemoPassword = "demo"
)

// StartDemo runs the demo devices of every registered platform (emulated by
// mockssh) on localhost and returns an App wired to them, with all data
// under a throw-away directory.
func StartDemo() (*App, func(), error) {
	dir, err := os.MkdirTemp("", "lazynet-demo-")
	if err != nil {
		return nil, nil, err
	}
	var servers []*mockssh.Server
	cleanup := func() {
		for _, s := range servers {
			_ = s.Close()
		}
		_ = os.RemoveAll(dir)
	}
	a, err := LoadFrom(config.PathsUnder(dir))
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	a.Demo = true
	// Never touch the real OS keyring from demo mode.
	a.Creds = creds.NewManager(0)
	a.Inv = &inventory.Inventory{}
	type spec struct {
		platform platform.ID
		demo     platform.Demo
	}
	var specs []spec
	for _, def := range platform.All() {
		for _, d := range def.Demo {
			specs = append(specs, spec{def.ID, d})
		}
	}
	kh, err := os.OpenFile(a.Paths.KnownHosts, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	defer kh.Close()
	for _, s := range specs {
		srv, err := mockssh.Start("127.0.0.1:0", mockssh.NewDevice(string(s.platform), s.demo.Name), DemoUser, DemoPassword)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		servers = append(servers, srv)
		host, port, _ := net.SplitHostPort(srv.Addr())
		p, _ := strconv.Atoi(port)
		a.Inv.Devices = append(a.Inv.Devices, inventory.Device{
			Name: s.demo.Name, Host: host, Port: p, Group: s.demo.Group, Username: DemoUser, Description: s.demo.Description,
		})
		fmt.Fprintln(kh, knownhosts.Line([]string{knownhosts.Normalize(srv.Addr())}, srv.HostKey))
	}
	a.Creds.Put(DemoUser, DemoPassword)
	return a, cleanup, nil
}
