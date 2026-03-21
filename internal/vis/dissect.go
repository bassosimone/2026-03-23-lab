//
// SPDX-License-Identifier: BSD-3-Clause
//
// Adapted from: https://github.com/ooni/netem/blob/3882eda4fb66244b28766ef8b02003515f476b37/dissect.go
//

package vis

import (
	"errors"
	"net/netip"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// DissectedPacket is a dissected IP packet. The zero value is invalid;
// use [DissectPacket] to create a new instance.
type DissectedPacket struct {
	// Packet is the underlying gopacket packet.
	Packet gopacket.Packet

	// IP is the network layer (either IPv4 or IPv6).
	IP gopacket.NetworkLayer

	// TCP is the possibly nil TCP layer.
	TCP *layers.TCP

	// UDP is the possibly nil UDP layer.
	UDP *layers.UDP
}

// ErrDissectShortPacket indicates the packet is too short.
var ErrDissectShortPacket = errors.New("vis: dissect: packet too short")

// ErrDissectNetwork indicates an unsupported network protocol.
var ErrDissectNetwork = errors.New("vis: dissect: unsupported network protocol")

// ErrDissectTransport indicates an unsupported transport protocol.
var ErrDissectTransport = errors.New("vis: dissect: unsupported transport protocol")

// DissectPacket parses a raw IP packet into its TCP/IP layers.
func DissectPacket(rawPacket []byte) (*DissectedPacket, error) {
	dp := &DissectedPacket{}

	// Sniff IP version from the first nibble.
	if len(rawPacket) < 1 {
		return nil, ErrDissectShortPacket
	}
	version := uint8(rawPacket[0]) >> 4

	// Parse the network layer.
	switch version {
	case 4:
		dp.Packet = gopacket.NewPacket(rawPacket, layers.LayerTypeIPv4, gopacket.Lazy)
		ipLayer := dp.Packet.Layer(layers.LayerTypeIPv4)
		if ipLayer == nil {
			return nil, ErrDissectNetwork
		}
		dp.IP = ipLayer.(*layers.IPv4)

	case 6:
		dp.Packet = gopacket.NewPacket(rawPacket, layers.LayerTypeIPv6, gopacket.Lazy)
		ipLayer := dp.Packet.Layer(layers.LayerTypeIPv6)
		if ipLayer == nil {
			return nil, ErrDissectNetwork
		}
		dp.IP = ipLayer.(*layers.IPv6)

	default:
		return nil, ErrDissectNetwork
	}

	// Parse the transport layer.
	switch dp.TransportProtocol() {
	case layers.IPProtocolTCP:
		dp.TCP = dp.Packet.Layer(layers.LayerTypeTCP).(*layers.TCP)

	case layers.IPProtocolUDP:
		dp.UDP = dp.Packet.Layer(layers.LayerTypeUDP).(*layers.UDP)

	default:
		return nil, ErrDissectTransport
	}

	return dp, nil
}

// SourceAddr returns the packet's source IP address.
func (dp *DissectedPacket) SourceAddr() netip.Addr {
	switch v := dp.IP.(type) {
	case *layers.IPv4:
		addr, _ := netip.AddrFromSlice(v.SrcIP)
		return addr
	case *layers.IPv6:
		addr, _ := netip.AddrFromSlice(v.SrcIP)
		return addr
	default:
		panic(ErrDissectNetwork)
	}
}

// SourcePort returns the packet's source port.
func (dp *DissectedPacket) SourcePort() uint16 {
	switch {
	case dp.TCP != nil:
		return uint16(dp.TCP.SrcPort)
	case dp.UDP != nil:
		return uint16(dp.UDP.SrcPort)
	default:
		panic(ErrDissectTransport)
	}
}

// DestinationAddr returns the packet's destination IP address.
func (dp *DissectedPacket) DestinationAddr() netip.Addr {
	switch v := dp.IP.(type) {
	case *layers.IPv4:
		addr, _ := netip.AddrFromSlice(v.DstIP)
		return addr
	case *layers.IPv6:
		addr, _ := netip.AddrFromSlice(v.DstIP)
		return addr
	default:
		panic(ErrDissectNetwork)
	}
}

// DestinationPort returns the packet's destination port.
func (dp *DissectedPacket) DestinationPort() uint16 {
	switch {
	case dp.TCP != nil:
		return uint16(dp.TCP.DstPort)
	case dp.UDP != nil:
		return uint16(dp.UDP.DstPort)
	default:
		panic(ErrDissectTransport)
	}
}

// TransportProtocol returns the packet's transport protocol.
func (dp *DissectedPacket) TransportProtocol() layers.IPProtocol {
	switch v := dp.IP.(type) {
	case *layers.IPv4:
		return v.Protocol
	case *layers.IPv6:
		return v.NextHeader
	default:
		panic(ErrDissectNetwork)
	}
}

// MatchesDestination reports whether the packet has the given
// protocol, destination address, and destination port.
func (dp *DissectedPacket) MatchesDestination(proto layers.IPProtocol, addr netip.Addr, port uint16) bool {
	if dp.TransportProtocol() != proto {
		return false
	}
	if dp.DestinationAddr() != addr {
		return false
	}
	return dp.DestinationPort() == port
}

// FlowHash returns a hash uniquely identifying the transport flow.
// Both directions of a flow produce the same hash.
func (dp *DissectedPacket) FlowHash() uint64 {
	switch {
	case dp.TCP != nil:
		return dp.TCP.TransportFlow().FastHash()
	case dp.UDP != nil:
		return dp.UDP.TransportFlow().FastHash()
	default:
		panic(ErrDissectTransport)
	}
}
