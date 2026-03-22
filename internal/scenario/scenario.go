// SPDX-License-Identifier: GPL-3.0-or-later

// Package scenario defines the simulated network topology
// for the censorship lab.
package scenario

import (
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/netip"

	"github.com/bassosimone/iss"
)

// Default returns the default lab [*iss.Scenario]. It extends the
// standard [iss.ScenarioV4] with a /download endpoint that serves
// a large response body for demonstrating throttling.
func Default() *iss.Scenario {
	return &iss.Scenario{
		ClientStack: iss.ClientStack{
			Addrs:    []netip.Addr{netip.MustParseAddr("130.192.91.211")},
			Resolver: netip.MustParseAddrPort("130.192.3.21:53"),
		},
		DNSServers: []iss.DNSServer{
			{
				Addrs:   []netip.Addr{netip.MustParseAddr("130.192.3.21")},
				Domains: []string{"giove.polito.it"},
			},
			{
				Addrs: []netip.Addr{
					netip.MustParseAddr("8.8.4.4"),
					netip.MustParseAddr("8.8.8.8"),
				},
				Domains: []string{"dns.google"},
				Aliases: []string{"dns.google.com"},
			},
		},
		HTTPServers: []iss.HTTPServer{
			{
				Addrs: []netip.Addr{
					netip.MustParseAddr("104.18.26.120"),
					netip.MustParseAddr("104.18.27.120"),
				},
				Domains: []string{"www.example.com", "example.com"},
				Aliases: []string{"www.example.org", "example.org"},
				Handler: exampleComHandler(),
			},
		},
	}
}

// downloadSize is the number of bytes served by /download.
const downloadSize = 1 << 20 // 1 MiB

// exampleComHandler returns an [http.Handler] that serves the default
// example.com page on / and a large body on /download.
func exampleComHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", iss.DefaultHTTPHandler)
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", downloadSize))
		io.CopyN(w, &randCharReader{rng: rand.New(rand.NewPCG(42, 0))}, downloadSize)
	})
	return mux
}

// printable is the set of characters used by [randCharReader].
const printable = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// randCharReader is an [io.Reader] that produces pseudorandom
// printable characters.
type randCharReader struct {
	rng *rand.Rand
}

// Read implements [io.Reader].
func (r *randCharReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = printable[r.rng.IntN(len(printable))]
	}
	return len(p), nil
}
