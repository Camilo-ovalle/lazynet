// Copyright 2026 Camilo Enrique Ovalle Ceron
// SPDX-License-Identifier: Apache-2.0

package catalyst

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Camilo-ovalle/lazynet/internal/parse"
	"github.com/Camilo-ovalle/lazynet/internal/platform"
	"github.com/Camilo-ovalle/lazynet/internal/platforms/cisco"
	"github.com/Camilo-ovalle/lazynet/internal/transport"
)

// VLAN change safety errors.
var (
	ErrTrunkPort     = errors.New("port is a trunk, or is operating as one: refusing to change it, this could take down an uplink")
	ErrNotAccessPort = errors.New("only access ports can be changed: static access, or dynamic auto/desirable while not operating as a trunk")
	ErrNotSwitchport = errors.New("interface is not a layer-2 switchport")
	ErrVlanNotFound  = errors.New("VLAN does not exist on the switch")
	ErrVlanInactive  = errors.New("VLAN is not active")
	ErrVlanRange     = errors.New("VLAN must be between 1 and 4094 (1002-1005 are reserved)")
	ErrVerifyFailed  = errors.New("verification failed: the port does not show the new VLAN")
)

// PlanVlanChange runs every pre-check with show commands only and builds the
// change request. Nothing is modified on the device.
//
// cisco.NormalizeInterface is the injection barrier: the port is strictly
// validated before it is interpolated into any command.
func (d *driver) PlanVlanChange(port string, vlan int) (*platform.VlanPlan, error) {
	if vlan < 1 || vlan > 4094 || (vlan >= 1002 && vlan <= 1005) {
		return nil, ErrVlanRange
	}
	name, err := cisco.NormalizeInterface(port, bare)
	if err != nil {
		return nil, err
	}
	out, err := d.s.Show("show interfaces " + name + " switchport")
	if err != nil {
		return nil, err
	}
	sp, ok := parse.ParseSwitchport(out)
	if !ok {
		return nil, fmt.Errorf("%s: %w", name, ErrNotSwitchport)
	}
	warnings, err := checkAccessPort(name, sp)
	if err != nil {
		return nil, err
	}
	vb, err := d.s.Show("show vlan brief")
	if err != nil {
		return nil, err
	}
	vlans := parse.ParseVlanBrief(vb)
	target, ok := vlans[vlan]
	if !ok {
		return nil, fmt.Errorf("VLAN %d: %w", vlan, ErrVlanNotFound)
	}
	if !target.Usable() {
		return nil, fmt.Errorf("VLAN %d (%s): %w", vlan, target.Status, ErrVlanInactive)
	}
	plan := &platform.VlanPlan{
		Port: name, PortMode: portMode(sp), CurrentVlan: sp.AccessVlan, CurrentVlanName: sp.VlanName,
		TargetVlan: vlan, TargetVlanName: target.Name, NoOp: sp.AccessVlan == vlan, Warnings: warnings,
	}
	if !plan.NoOp {
		plan.Change = transport.NewConfigChange("interface "+name, fmt.Sprintf("switchport access vlan %d", vlan))
		plan.Save = d.SaveRequest()
	}
	return plan, nil
}

// checkAccessPort decides whether the access VLAN of a port may be changed,
// from the modes the switch reports. Allowed:
//   - static access ports, up or down;
//   - dynamic auto/desirable ports (the Catalyst default) that are down or
//     running as an access port. The change only sets the access VLAN; the
//     port keeps negotiating with DTP, which the returned warning explains.
//
// Everything else is refused: configured trunks, any port operating as a
// trunk (an uplink, an AP, a phone with a trunk), bundle members and modes
// lazynet doesn't know (private VLAN, dot1q tunnel…).
func checkAccessPort(name string, sp parse.Switchport) (warnings []string, err error) {
	admin, oper := strings.ToLower(strings.TrimSpace(sp.AdminMode)), strings.ToLower(strings.TrimSpace(sp.OperMode))
	switch {
	case sp.ConfiguredTrunk() || sp.OperatingTrunk():
		return nil, fmt.Errorf("%s (mode %s): %w", name, portMode(sp), ErrTrunkPort)
	case admin != parse.ModeStaticAccess && !sp.Dynamic(),
		oper != parse.ModeStaticAccess && oper != parse.ModeDown:
		return nil, fmt.Errorf("%s (mode %s): %w", name, portMode(sp), ErrNotAccessPort)
	}
	if sp.Dynamic() {
		warnings = append(warnings, fmt.Sprintf("%s is in %s mode: it keeps negotiating with DTP. "+
			"The new access VLAN applies while it runs as an access port; if its neighbour negotiates a trunk, it becomes a trunk.",
			name, admin))
	}
	return warnings, nil
}

// portMode describes the configured and operational modes, e.g.
// "dynamic auto, operational: down".
func portMode(sp parse.Switchport) string {
	m := strings.ToLower(sp.AdminMode)
	if sp.OperMode != "" && !strings.EqualFold(sp.OperMode, sp.AdminMode) {
		m += ", operational: " + strings.ToLower(sp.OperMode)
	}
	return m
}

// ApplyVlanChange sends the (already confirmed) plan and verifies the result
// by reading the switchport state back.
func (d *driver) ApplyVlanChange(a platform.Applier, plan *platform.VlanPlan) (output string, err error) {
	if plan.NoOp || plan.Change == nil {
		return "", nil
	}
	output, err = a.Apply(plan.Change)
	if err != nil {
		return output, err
	}
	out, err := a.Show("show interfaces " + plan.Port + " switchport")
	if err != nil {
		return output, fmt.Errorf("change sent but verification failed: %w", err)
	}
	sp, _ := parse.ParseSwitchport(out)
	if sp.AccessVlan != plan.TargetVlan {
		return output, fmt.Errorf("%w (port shows VLAN %d)", ErrVerifyFailed, sp.AccessVlan)
	}
	return output, nil
}

// SaveRequest builds the change request that persists running-config.
func (d *driver) SaveRequest() *transport.ChangeRequest {
	return transport.NewExecChange("write memory")
}
