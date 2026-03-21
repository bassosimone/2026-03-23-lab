//
// SPDX-License-Identifier: BSD-3-Clause
//
// Adapted from: https://github.com/ooni/netem/blob/3882eda4fb66244b28766ef8b02003515f476b37/dpiblock.go
//

package vis

import (
	"net/netip"

	"github.com/bassosimone/uis"
	"github.com/google/gopacket/layers"
	"github.com/miekg/dns"
)

// DPIDNSRule is a [DPIRule] that injects a spoofed DNS response
// when it sees a query for a given domain. If [Addresses] is
// empty, the spoofed response contains NXDOMAIN. The zero value
// is invalid; fill all fields marked as MANDATORY.
type DPIDNSRule struct {
	// Addresses contains OPTIONAL IP addresses to include
	// in the spoofed response. If empty, replies with NXDOMAIN.
	Addresses []netip.Addr

	// Domain is the MANDATORY domain to match.
	Domain string
}

var _ DPIRule = &DPIDNSRule{}

// Filter implements [DPIRule].
func (r *DPIDNSRule) Filter(
	direction DPIDirection, packet *DissectedPacket, ix *uis.Internet) (DPIPolicy, bool) {
	// Only inspect client-to-server traffic.
	if direction != DPIDirectionClientToServer {
		return DPIPolicy{}, false
	}

	// Only inspect UDP packets to port 53.
	if packet.TransportProtocol() != layers.IPProtocolUDP {
		return DPIPolicy{}, false
	}
	if packet.DestinationPort() != 53 {
		return DPIPolicy{}, false
	}

	// Try to parse the DNS request.
	request := &dns.Msg{}
	if err := request.Unpack(packet.UDP.Payload); err != nil {
		return DPIPolicy{}, false
	}
	if len(request.Question) < 1 {
		return DPIPolicy{}, false
	}
	question := request.Question[0]
	if question.Name != dns.CanonicalName(r.Domain) {
		return DPIPolicy{}, false
	}

	// Build the spoofed DNS response.
	rawResponse, err := r.buildResponse(request, question)
	if err != nil {
		return DPIPolicy{}, false
	}

	// Build and inject the spoofed UDP datagram.
	spoofed, err := reflectUDPWithPayload(packet, rawResponse)
	if err == nil {
		ix.Deliver(uis.VNICFrame{Packet: spoofed})
	}

	return DPIPolicy{}, true
}

// buildResponse constructs a DNS response message.
func (r *DPIDNSRule) buildResponse(request *dns.Msg, question dns.Question) ([]byte, error) {
	response := &dns.Msg{}
	response.SetReply(request)
	response.RecursionAvailable = true

	// Build A/AAAA records from configured addresses.
	for _, addr := range r.Addresses {
		if addr.Is4() {
			response.Answer = append(response.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				A: addr.AsSlice(),
			})
		} else {
			response.Answer = append(response.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				AAAA: addr.AsSlice(),
			})
		}
	}

	// If no addresses were configured, respond with NXDOMAIN.
	if len(response.Answer) <= 0 {
		response.Rcode = dns.RcodeNameError
	}

	return response.Pack()
}
