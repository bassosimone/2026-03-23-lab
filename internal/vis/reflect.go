//
// SPDX-License-Identifier: BSD-3-Clause
//
// Adapted from: https://github.com/ooni/netem/blob/3882eda4fb66244b28766ef8b02003515f476b37/dissect.go
//

package vis

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// reflectedIP holds a reflected IP layer ready for serialization.
type reflectedIP struct {
	// serializable is the layer to pass to SerializeLayers.
	serializable gopacket.SerializableLayer

	// network is the layer to pass to SetNetworkLayerForChecksum.
	network gopacket.NetworkLayer
}

// reflectIPLayer constructs a reflected IP layer by swapping source
// and destination addresses. Supports both IPv4 and IPv6.
func reflectIPLayer(packet *DissectedPacket) (*reflectedIP, error) {
	switch v := packet.IP.(type) {
	case *layers.IPv4:
		reflected := &layers.IPv4{
			Version:  4,
			Id:       v.Id,
			TTL:      60,
			Protocol: v.Protocol,
			SrcIP:    v.DstIP,
			DstIP:    v.SrcIP,
		}
		return &reflectedIP{
			serializable: reflected,
			network:      reflected,
		}, nil

	case *layers.IPv6:
		reflected := &layers.IPv6{
			Version:    6,
			HopLimit:   60,
			NextHeader: v.NextHeader,
			SrcIP:      v.DstIP,
			DstIP:      v.SrcIP,
		}
		return &reflectedIP{
			serializable: reflected,
			network:      reflected,
		}, nil

	default:
		return nil, ErrDissectNetwork
	}
}

// reflectTCPWithFlags constructs a spoofed TCP segment by swapping source
// and destination fields of the given packet. The setter callback configures
// the TCP flags (e.g., RST, FIN|ACK) before serialization.
func reflectTCPWithFlags(packet *DissectedPacket, setter func(tcp *layers.TCP)) ([]byte, error) {
	// Make sure the transport is TCP.
	if packet.TCP == nil {
		return nil, ErrDissectTransport
	}

	// Reflect IP layer.
	ip, err := reflectIPLayer(packet)
	if err != nil {
		return nil, err
	}

	// Reflect TCP layer.
	reflectedTCP := &layers.TCP{
		SrcPort: packet.TCP.DstPort,
		DstPort: packet.TCP.SrcPort,
		Seq:     packet.TCP.Ack,
		Ack:     packet.TCP.Seq + 1,
		Window:  packet.TCP.Window,
	}

	setter(reflectedTCP)

	reflectedTCP.SetNetworkLayerForChecksum(ip.network)
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	if err := gopacket.SerializeLayers(buf, opts, ip.serializable, reflectedTCP); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// reflectUDPWithPayload constructs a spoofed UDP datagram by swapping source
// and destination fields of the given packet and replacing the payload.
func reflectUDPWithPayload(packet *DissectedPacket, rawPayload []byte) ([]byte, error) {
	// Make sure the transport is UDP.
	if packet.UDP == nil {
		return nil, ErrDissectTransport
	}

	// Reflect IP layer.
	ip, err := reflectIPLayer(packet)
	if err != nil {
		return nil, err
	}

	// Reflect UDP layer.
	reflectedUDP := &layers.UDP{
		SrcPort: packet.UDP.DstPort,
		DstPort: packet.UDP.SrcPort,
	}

	reflectedUDP.SetNetworkLayerForChecksum(ip.network)
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	payload := gopacket.Payload(rawPayload)
	if err := gopacket.SerializeLayers(buf, opts, ip.serializable, reflectedUDP, payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
