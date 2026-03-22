// SPDX-License-Identifier: GPL-3.0-or-later

package dissector

import (
	"strings"
)

// HTTPDetail contains a dissected HTTP/1.x message.
type HTTPDetail struct {
	// FirstLine is the request line or status line.
	FirstLine string `json:"first_line"`

	// Headers contains all HTTP headers as raw strings.
	Headers []string `json:"headers"`
}

// httpMethods are the HTTP methods we recognize in the first line.
var httpMethods = []string{"GET ", "POST ", "PUT ", "DELETE ", "HEAD ", "OPTIONS ", "PATCH "}

// dissectHTTP attempts to parse a TCP payload as an HTTP/1.x message.
// Returns nil if the payload does not look like HTTP.
func dissectHTTP(payload []byte) *HTTPDetail {
	if len(payload) <= 0 {
		return nil
	}

	s := string(payload)
	isHTTP := strings.HasPrefix(s, "HTTP/")

	if !isHTTP {
		for _, method := range httpMethods {
			if strings.HasPrefix(s, method) {
				isHTTP = true
				break
			}
		}
	}

	if !isHTTP {
		return nil
	}

	lines := strings.Split(s, "\r\n")
	detail := &HTTPDetail{FirstLine: lines[0]}

	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		detail.Headers = append(detail.Headers, line)
	}

	return detail
}

// httpInfoLine builds a Wireshark-style info string for an HTTP message.
func httpInfoLine(d *HTTPDetail) string {
	return d.FirstLine
}
