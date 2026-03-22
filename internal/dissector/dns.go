// SPDX-License-Identifier: GPL-3.0-or-later

package dissector

import (
	"fmt"
	"strings"

	"github.com/miekg/dns"
)

// DNSDetail contains a dissected DNS message.
type DNSDetail struct {
	// TransactionID is the DNS message ID.
	TransactionID uint16 `json:"transaction_id"`

	// QR is true for a response, false for a query.
	QR bool `json:"qr"`

	// Opcode is the DNS opcode (e.g., 0 for standard query).
	Opcode int `json:"opcode"`

	// Rcode is the response code name (e.g., "NOERROR", "NXDOMAIN").
	Rcode string `json:"rcode"`

	// Questions is the list of DNS questions.
	Questions []DNSQuestion `json:"questions"`

	// Answers is the list of DNS answer records.
	Answers []string `json:"answers,omitempty"`
}

// DNSQuestion represents a single DNS question.
type DNSQuestion struct {
	// Name is the queried domain name.
	Name string `json:"name"`

	// Type is the query type name (e.g., "A", "AAAA").
	Type string `json:"type"`
}

// dissectDNS attempts to parse a UDP payload as a DNS message.
// Returns nil if the payload is not a valid DNS message.
func dissectDNS(payload []byte) *DNSDetail {
	var msg dns.Msg
	if err := msg.Unpack(payload); err != nil {
		return nil
	}

	detail := &DNSDetail{
		TransactionID: msg.Id,
		QR:            msg.Response,
		Opcode:        int(msg.Opcode),
		Rcode:         dns.RcodeToString[msg.Rcode],
	}

	// Parse questions.
	for _, q := range msg.Question {
		detail.Questions = append(detail.Questions, DNSQuestion{
			Name: q.Name,
			Type: dns.TypeToString[q.Qtype],
		})
	}

	// Parse answer records.
	for _, rr := range msg.Answer {
		detail.Answers = append(detail.Answers, formatRR(rr))
	}

	return detail
}

// formatRR formats a DNS resource record as a compact string
// (e.g., "www.example.com. A 104.18.26.120").
func formatRR(rr dns.RR) string {
	hdr := rr.Header()
	typeName := dns.TypeToString[hdr.Rrtype]

	switch v := rr.(type) {
	case *dns.A:
		return fmt.Sprintf("%s %s %s", hdr.Name, typeName, v.A.String())

	case *dns.AAAA:
		return fmt.Sprintf("%s %s %s", hdr.Name, typeName, v.AAAA.String())

	case *dns.CNAME:
		return fmt.Sprintf("%s %s %s", hdr.Name, typeName, v.Target)

	case *dns.MX:
		return fmt.Sprintf("%s %s %d %s", hdr.Name, typeName, v.Preference, v.Mx)

	case *dns.NS:
		return fmt.Sprintf("%s %s %s", hdr.Name, typeName, v.Ns)

	case *dns.PTR:
		return fmt.Sprintf("%s %s %s", hdr.Name, typeName, v.Ptr)

	case *dns.TXT:
		return fmt.Sprintf("%s %s %s", hdr.Name, typeName, strings.Join(v.Txt, " "))

	default:
		return rr.String()
	}
}

// dnsInfoLine builds a Wireshark-style info string for a DNS message.
func dnsInfoLine(d *DNSDetail) string {
	kind := "query"
	if d.QR {
		kind = "response"
	}

	if len(d.Questions) > 0 {
		q := d.Questions[0]
		info := fmt.Sprintf("DNS %s: %s %s", kind, q.Name, q.Type)

		// For responses, append the first answer or the rcode.
		if d.QR {
			if len(d.Answers) > 0 {
				info += " → " + d.Answers[0]
			} else {
				info += " → " + d.Rcode
			}
		}

		return info
	}

	return fmt.Sprintf("DNS %s", kind)
}
