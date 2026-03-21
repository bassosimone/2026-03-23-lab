//
// SPDX-License-Identifier: BSD-3-Clause
//
// Adapted from: https://github.com/ooni/netem/blob/3882eda4fb66244b28766ef8b02003515f476b37/dpiblock.go
// Adapted from: https://github.com/ooni/netem/blob/3882eda4fb66244b28766ef8b02003515f476b37/dpidrop.go
// Adapted from: https://github.com/ooni/netem/blob/3882eda4fb66244b28766ef8b02003515f476b37/dpithrottle.go
//

package vis

import (
	"bytes"
	"net/netip"
	"time"

	"github.com/bassosimone/uis"
	"github.com/google/gopacket/layers"
)

// DPITCPAction is the action a [DPITCPRule] takes on match.
type DPITCPAction int

const (
	// DPITCPActionDrop drops the matched packet.
	//
	// Implies also dropping any subsequent packet on the flow.
	DPITCPActionDrop DPITCPAction = iota

	// DPITCPActionReset injects a spoofed TCP RST segment.
	//
	// Implies also dropping any subsequent packet on the flow.
	DPITCPActionReset

	// DPITCPActionThrottle adds delay and packet loss to the
	// matched packet and to any subsequent packet.
	DPITCPActionThrottle
)

// DPITCPRule is a [DPIRule] that matches TCP traffic by server
// endpoint and/or payload string, then applies an action (drop,
// reset, or throttle).
//
// Match criteria are conjunctive: all non-zero fields must match.
// At least one match field should be set.
type DPITCPRule struct {
	// Action is the action to take on match.
	Action DPITCPAction

	// Contains is an OPTIONAL string to match in the TCP payload.
	//
	// Empty means match any payload.
	Contains string

	// Delay is the extra delay for [DPITCPActionThrottle].
	Delay time.Duration

	// PLR is the extra packet loss rate for [DPITCPActionThrottle] (0.0–1.0).
	PLR float64

	// ServerAddr is the OPTIONAL server IP address to match.
	//
	// Zero value means match any destination IP.
	ServerAddr netip.Addr

	// ServerPort is the OPTIONAL server port to match.
	//
	// Zero means match any destination port.
	ServerPort uint16
}

var _ DPIRule = &DPITCPRule{}

// Filter implements [DPIRule].
func (r *DPITCPRule) Filter(
	direction DPIDirection, packet *DissectedPacket, ix *uis.Internet) (DPIPolicy, bool) {
	// Only inspect client-to-server TCP traffic.
	if direction != DPIDirectionClientToServer {
		return DPIPolicy{}, false
	}
	if packet.TransportProtocol() != layers.IPProtocolTCP {
		return DPIPolicy{}, false
	}

	// Match server endpoint if configured.
	if r.ServerAddr.IsValid() && packet.DestinationAddr() != r.ServerAddr {
		return DPIPolicy{}, false
	}
	if r.ServerPort != 0 && packet.DestinationPort() != r.ServerPort {
		return DPIPolicy{}, false
	}

	// Match payload string if configured.
	if r.Contains != "" && !bytes.Contains(packet.TCP.Payload, []byte(r.Contains)) {
		return DPIPolicy{}, false
	}

	// Apply the action.
	switch r.Action {
	case DPITCPActionDrop:
		return DPIPolicy{PLR: 1.0}, true

	case DPITCPActionReset:
		spoofed, err := reflectTCPWithFlags(packet, func(tcp *layers.TCP) {
			tcp.RST = true
		})
		if err == nil {
			ix.Deliver(uis.VNICFrame{Packet: spoofed})
		}
		return DPIPolicy{PLR: 1.0}, true

	case DPITCPActionThrottle:
		return DPIPolicy{Delay: r.Delay, PLR: r.PLR}, true

	default:
		return DPIPolicy{}, false
	}
}
