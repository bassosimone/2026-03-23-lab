//
// SPDX-License-Identifier: BSD-3-Clause
//
// Adapted from: https://github.com/ooni/netem/blob/3882eda4fb66244b28766ef8b02003515f476b37/dpiengine.go
//

package vis

import (
	"net/netip"
	"sync"
	"time"

	"github.com/bassosimone/uis"
	"github.com/google/gopacket/layers"
)

// DPIDirection is the direction of a packet within a flow.
type DPIDirection int

const (
	// DPIDirectionClientToServer indicates a packet flowing from
	// the client to the server. The client is the endpoint that
	// sends the first packet in the flow.
	DPIDirectionClientToServer DPIDirection = iota

	// DPIDirectionServerToClient indicates a packet flowing from
	// the server to the client. The client is the endpoint that
	// sends the first packet in the flow.
	DPIDirectionServerToClient
)

// DPIPolicy describes the routing action to apply to a matched packet.
// Rules that need to inject spoofed packets (e.g., TCP RST, fake DNS
// responses) do so directly via [*uis.Internet] during [DPIRule.Filter].
//
// To drop a packet, set PLR to 1.0. A PLR >= 1.0 guarantees the
// packet will be discarded by the router.
type DPIPolicy struct {
	// Delay is extra delay to add before delivering the packet.
	Delay time.Duration

	// PLR is extra packet loss rate to apply (0.0–1.0).
	// A value >= 1.0 causes unconditional drop.
	PLR float64
}

// DPIRule is a deep packet inspection rule. Implementations inspect
// a [*DissectedPacket] and return a [DPIPolicy] if the rule matches.
// Rules that need to inject spoofed packets do so directly via the
// provided [*uis.Internet].
type DPIRule interface {
	// Filter inspects a packet and returns a policy if matched.
	// Rules may inject spoofed packets directly via ix.
	Filter(direction DPIDirection, packet *DissectedPacket, ix *uis.Internet) (DPIPolicy, bool)
}

// DPIEngine is a deep packet inspection engine that tracks flows
// and applies [DPIRule] to packets. The zero value is invalid;
// construct using [NewDPIEngine].
type DPIEngine struct {
	// flows tracks per-flow state.
	flows map[uint64]*dpiFlow

	// mu protects flows and rules.
	mu sync.Mutex

	// rules contains the registered rules.
	rules []DPIRule
}

// NewDPIEngine creates a new [*DPIEngine].
func NewDPIEngine() *DPIEngine {
	return &DPIEngine{
		flows: map[uint64]*dpiFlow{},
		rules: nil,
	}
}

// AddRule appends a [DPIRule] to the engine. Rules are evaluated in order; the
// first match wins. This method is concurrency safe.
func (de *DPIEngine) AddRule(rule DPIRule) {
	de.mu.Lock()
	defer de.mu.Unlock()
	de.rules = append(de.rules, rule)
}

// ClearRules removes all rules. Previously matched flows keep their
// cached policy until they expire. This method is concurrency safe.
func (de *DPIEngine) ClearRules() {
	de.mu.Lock()
	defer de.mu.Unlock()
	de.rules = nil
}

// Inspect applies DPI to a raw IP packet. It returns a policy and true if
// a rule matched, or the zero policy and false otherwise.
func (de *DPIEngine) Inspect(rawPacket []byte, ix *uis.Internet) (DPIPolicy, bool) {
	// Parse the packet; silently ignore packets we cannot dissect.
	packet, err := DissectPacket(rawPacket)
	if err != nil {
		return DPIPolicy{}, false
	}

	// Look up or create the flow record.
	flow := de.getFlow(packet)

	// Lock the flow while we process this packet.
	flow.mu.Lock()
	defer flow.mu.Unlock()

	// Increment number of packets per flow.
	flow.numPackets++

	// If we already determined a policy for this flow, reuse it.
	if flow.matched {
		return flow.policy, true
	}

	// Stop inspecting after 10 packets to bound work per flow.
	const maxPackets = 10
	if flow.numPackets >= maxPackets {
		return DPIPolicy{}, false
	}

	// Determine packet direction within the flow.
	direction := flow.directionLocked(packet)

	// Evaluate rules in order; first match wins. Hold the lock to
	// avoid racing with someone adding new filtering rules.
	de.mu.Lock()
	defer de.mu.Unlock()
	for _, rule := range de.rules {
		policy, match := rule.Filter(direction, packet, ix)
		if match {
			flow.policy = policy
			flow.matched = true
			return policy, true
		}
	}

	return DPIPolicy{}, false
}

// getFlow returns the flow record for the given packet, creating a new
// one if none exists or the existing one is stale.
func (de *DPIEngine) getFlow(packet *DissectedPacket) *dpiFlow {
	de.mu.Lock()
	defer de.mu.Unlock()

	// Expire flows after 30 seconds of silence.
	const maxSilence = 30 * time.Second
	fh := packet.FlowHash()
	flow := de.flows[fh]
	if flow == nil || time.Since(flow.updated) > maxSilence {
		flow = newDPIFlow(packet)
		de.flows[fh] = flow
	}
	flow.updated = time.Now()
	return flow
}

// dpiFlow tracks per-flow state for the DPI engine.
type dpiFlow struct {
	// destAddr is the destination IP of the first packet.
	destAddr netip.Addr

	// destPort is the destination port of the first packet.
	destPort uint16

	// mu protects mutable flow state.
	mu sync.Mutex

	// matched is true once a rule has matched this flow.
	matched bool

	// numPackets counts packets seen in either direction.
	numPackets int64

	// policy is the cached policy (valid when matched is true).
	policy DPIPolicy

	// protocol is the transport protocol (TCP or UDP).
	protocol layers.IPProtocol

	// sourceAddr is the source IP of the first packet.
	sourceAddr netip.Addr

	// sourcePort is the source port of the first packet.
	sourcePort uint16

	// updated is the last time this flow was seen.
	updated time.Time
}

// newDPIFlow creates a new flow record from the first packet.
func newDPIFlow(packet *DissectedPacket) *dpiFlow {
	return &dpiFlow{
		destAddr:   packet.DestinationAddr(),
		destPort:   packet.DestinationPort(),
		protocol:   packet.TransportProtocol(),
		sourceAddr: packet.SourceAddr(),
		sourcePort: packet.SourcePort(),
		updated:    time.Now(),
	}
}

// directionLocked returns the direction of the packet relative
// to the flow's original client→server direction.
//
// The caller must have already locked the flow when calling this method.
func (df *dpiFlow) directionLocked(packet *DissectedPacket) DPIDirection {
	if packet.MatchesDestination(df.protocol, df.destAddr, df.destPort) {
		return DPIDirectionClientToServer
	}
	return DPIDirectionServerToClient
}
