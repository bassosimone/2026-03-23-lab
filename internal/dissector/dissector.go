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

	// Detail contains the full header breakdown for the detail pane.
	Detail *PacketDetail `json:"detail"`
}

// PacketDetail contains the full header fields for the detail pane.
type PacketDetail struct {
	// IP is the network layer header detail.
	IP *IPDetail `json:"ip"`

	// TCP is the transport layer header detail (nil for UDP).
	TCP *TCPDetail `json:"tcp,omitempty"`

	// UDP is the transport layer header detail (nil for TCP).
	UDP *UDPDetail `json:"udp,omitempty"`

	// DNS is the application layer DNS detail (nil if not DNS).
	DNS *DNSDetail `json:"dns,omitempty"`

	// HTTP is the application layer HTTP detail (nil if not HTTP).
	HTTP *HTTPDetail `json:"http,omitempty"`
}

// IPDetail contains IPv4 header fields.
type IPDetail struct {
	Version    uint8  `json:"version"`
	IHL        uint8  `json:"ihl"`
	TOS        uint8  `json:"tos"`
	TotalLen   uint16 `json:"total_length"`
	ID         uint16 `json:"id"`
	FlagDF     bool   `json:"flag_df"`
	FlagMF     bool   `json:"flag_mf"`
	FragOffset uint16 `json:"frag_offset"`
	TTL        uint8  `json:"ttl"`
	Protocol   uint8  `json:"protocol"`
	Checksum   uint16 `json:"checksum"`
	Src        string `json:"src"`
	Dst        string `json:"dst"`
}

// TCPDetail contains TCP header fields.
type TCPDetail struct {
	SrcPort    uint16 `json:"src_port"`
	DstPort    uint16 `json:"dst_port"`
	Seq        uint32 `json:"seq"`
	Ack        uint32 `json:"ack"`
	DataOffset uint8  `json:"data_offset"`
	FlagSYN    bool   `json:"flag_syn"`
	FlagACK    bool   `json:"flag_ack"`
	FlagFIN    bool   `json:"flag_fin"`
	FlagRST    bool   `json:"flag_rst"`
	FlagPSH    bool   `json:"flag_psh"`
	FlagURG    bool   `json:"flag_urg"`
	Window     uint16 `json:"window"`
	Checksum   uint16 `json:"checksum"`
	Urgent     uint16 `json:"urgent"`
	PayloadLen int    `json:"payload_length"`
}

// UDPDetail contains UDP header fields.
type UDPDetail struct {
	SrcPort    uint16 `json:"src_port"`
	DstPort    uint16 `json:"dst_port"`
	Length     uint16 `json:"length"`
	Checksum   uint16 `json:"checksum"`
	PayloadLen int    `json:"payload_length"`
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

	// Build detail.
	// TODO(bassosimone): the detail pane currently does not dissect IPv6
	// headers — only IPv4 is supported. The summary fields (src, dst, etc.)
	// work for both because they use DissectedPacket accessors.
	detail := &PacketDetail{}
	if ipv4, ok := dp.IP.(*layers.IPv4); ok {
		detail.IP = &IPDetail{
			Version:    ipv4.Version,
			IHL:        ipv4.IHL,
			TOS:        ipv4.TOS,
			TotalLen:   ipv4.Length,
			ID:         ipv4.Id,
			FlagDF:     ipv4.Flags&layers.IPv4DontFragment != 0,
			FlagMF:     ipv4.Flags&layers.IPv4MoreFragments != 0,
			FragOffset: ipv4.FragOffset,
			TTL:        ipv4.TTL,
			Protocol:   uint8(ipv4.Protocol),
			Checksum:   ipv4.Checksum,
			Src:        ipv4.SrcIP.String(),
			Dst:        ipv4.DstIP.String(),
		}
	}
	s.Detail = detail

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
		detail.TCP = &TCPDetail{
			SrcPort:    uint16(dp.TCP.SrcPort),
			DstPort:    uint16(dp.TCP.DstPort),
			Seq:        dp.TCP.Seq,
			Ack:        dp.TCP.Ack,
			DataOffset: dp.TCP.DataOffset,
			FlagSYN:    dp.TCP.SYN,
			FlagACK:    dp.TCP.ACK,
			FlagFIN:    dp.TCP.FIN,
			FlagRST:    dp.TCP.RST,
			FlagPSH:    dp.TCP.PSH,
			FlagURG:    dp.TCP.URG,
			Window:     dp.TCP.Window,
			Checksum:   dp.TCP.Checksum,
			Urgent:     dp.TCP.Urgent,
			PayloadLen: len(dp.TCP.Payload),
		}

		// Attempt HTTP dissection on port 80.
		if dp.TCP.SrcPort == 80 || dp.TCP.DstPort == 80 {
			if h := dissectHTTP(dp.TCP.Payload); h != nil {
				detail.HTTP = h
				s.Protocol = "HTTP"
				s.Info = httpInfoLine(h)
			}
		}

	case layers.IPProtocolUDP:
		s.Protocol = "UDP"
		s.Info = udpInfo(s)
		detail.UDP = &UDPDetail{
			SrcPort:    uint16(dp.UDP.SrcPort),
			DstPort:    uint16(dp.UDP.DstPort),
			Length:     dp.UDP.Length,
			Checksum:   dp.UDP.Checksum,
			PayloadLen: len(dp.UDP.Payload),
		}

		// Attempt DNS dissection when either port is 53.
		if dp.UDP.SrcPort == 53 || dp.UDP.DstPort == 53 {
			if d := dissectDNS(dp.UDP.Payload); d != nil {
				detail.DNS = d
				s.Protocol = "DNS"
				s.Info = dnsInfoLine(d)
			}
		}
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
