// Package device implements per-platform drivers (Catalyst, Nexus, ASA):
// detection, information gathering and the guarded VLAN change workflow.
package device

import "fmt"

// Platform identifies a Cisco device family.
type Platform string

const (
	Unknown  Platform = ""
	Catalyst Platform = "catalyst" // IOS and IOS-XE switches
	Nexus    Platform = "nexus"    // NX-OS
	ASA      Platform = "asa"      // Adaptive Security Appliance
)

// ParsePlatform validates a user-provided platform name.
func ParsePlatform(s string) (Platform, error) {
	switch Platform(s) {
	case Catalyst, Nexus, ASA, Unknown:
		return Platform(s), nil
	}
	return Unknown, fmt.Errorf("unknown platform %q (use catalyst, nexus or asa)", s)
}

// Title is the human name of the platform.
func (p Platform) Title() string {
	switch p {
	case Catalyst:
		return "Catalyst"
	case Nexus:
		return "Nexus"
	case ASA:
		return "ASA"
	}
	return "Unknown"
}
