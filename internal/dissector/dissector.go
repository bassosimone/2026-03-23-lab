// SPDX-License-Identifier: GPL-3.0-or-later

// Package dissector produces JSON-serializable packet summaries
// from already-dissected packet log entries.
package dissector

import (
	"fmt"
	"strings"

	"github.com/bassosimone/2026-03-23-lab/internal/pktlog"
	"github.com/bassosimone/2026-03-23-lab/internal/vis"
	"github.com/google/gopacket/layers"
)

// PacketSummary is a JSON-serializable summary of a single packet.
type PacketSummary struct {
	// Number is the 1-based packet index within the response.
	Number int `json:"number"`

	// Event is "entered" or "delivered".
	Event string `json:"event"`

	// Time is the event timestamp as a string.
	Time string `json:"time"`

	// Source IP address.
	Src string `json:"src"`

	// Destination IP address.
	Dst string `json:"dst"`

	// Protocol name ("TCP" or "UDP").
	Protocol string `json:"protocol"`

	// Length is the total IP packet length in bytes.
	Length int `json:"length"`

	// SrcPort is the source transport port.
	SrcPort uint16 `json:"src_port"`

	// DstPort is the destination transport port.
	DstPort uint16 `json:"dst_port"`

	// Flags contains TCP flags (e.g., "SYN", "SYN ACK", "RST").
	// Empty for UDP packets.
	Flags string `json:"flags,omitempty"`

	// Seq is the TCP sequence number. Omitted for UDP.
	Seq *uint32 `json:"seq,omitempty"`

	// Ack is the TCP acknowledgment number. Omitted for UDP
	// and when the ACK flag is not set.
	Ack *uint32 `json:"ack,omitempty"`

	// Info is a Wireshark-style one-line summary.
	Info string `json:"info"`
}

// Summarize produces a [*PacketSummary] from an already-dissected
// packet log entry. The number parameter is the 1-based index.
func Summarize(entry pktlog.DissectedEntry, number int) *PacketSummary {
	dp := entry.Packet
	s := &PacketSummary{
		Number:   number,
		Event:    eventString(entry.Event),
		Time:     entry.Time.Format("15:04:05.000000"),
		Src:      dp.SourceAddr().String(),
		Dst:      dp.DestinationAddr().String(),
		Length:   len(entry.RawPacket),
		SrcPort:  dp.SourcePort(),
		DstPort:  dp.DestinationPort(),
	}

	switch dp.TransportProtocol() {
	case layers.IPProtocolTCP:
		s.Protocol = "TCP"
		s.Flags = tcpFlags(dp.TCP)
		seq := dp.TCP.Seq
		s.Seq = &seq
		if dp.TCP.ACK {
			ack := dp.TCP.Ack
			s.Ack = &ack
		}
		s.Info = tcpInfo(s)

	case layers.IPProtocolUDP:
		s.Protocol = "UDP"
		s.Info = udpInfo(s)
	}

	return s
}

// tcpFlags returns a space-separated string of active TCP flags.
func tcpFlags(tcp *layers.TCP) string {
	var flags []string
	if tcp.SYN {
		flags = append(flags, "SYN")
	}
	if tcp.ACK {
		flags = append(flags, "ACK")
	}
	if tcp.FIN {
		flags = append(flags, "FIN")
	}
	if tcp.RST {
		flags = append(flags, "RST")
	}
	if tcp.PSH {
		flags = append(flags, "PSH")
	}
	if tcp.URG {
		flags = append(flags, "URG")
	}
	return strings.Join(flags, " ")
}

// tcpInfo builds a Wireshark-style info line for a TCP packet.
func tcpInfo(s *PacketSummary) string {
	info := fmt.Sprintf("%d \u2192 %d [%s] Seq=%d", s.SrcPort, s.DstPort, s.Flags, *s.Seq)
	if s.Ack != nil {
		info += fmt.Sprintf(" Ack=%d", *s.Ack)
	}
	payloadLen := s.Length - 40 // approximate: IP(20) + TCP(20) header
	if payloadLen < 0 {
		payloadLen = 0
	}
	info += fmt.Sprintf(" Len=%d", payloadLen)
	return info
}

// udpInfo builds a Wireshark-style info line for a UDP packet.
func udpInfo(s *PacketSummary) string {
	payloadLen := s.Length - 28 // IP(20) + UDP(8) header
	if payloadLen < 0 {
		payloadLen = 0
	}
	return fmt.Sprintf("%d \u2192 %d Len=%d", s.SrcPort, s.DstPort, payloadLen)
}

// eventString converts a [vis.PacketEvent] to a string.
func eventString(event vis.PacketEvent) string {
	switch event {
	case vis.PacketEntered:
		return "entered"
	case vis.PacketDelivered:
		return "delivered"
	default:
		return "unknown"
	}
}
