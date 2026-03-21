// SPDX-License-Identifier: GPL-3.0-or-later

package command

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/bassosimone/dnscodec"
	"github.com/bassosimone/dnsoverhttps"
	"github.com/bassosimone/minest"
	"github.com/bassosimone/vflag"
	"github.com/miekg/dns"
)

// runDig implements the "dig" command: sends a DNS query and
// prints the response in dig-like format.
func (r *Runner) runDig(ctx context.Context, params *Params) error {
	// Parse flags.
	fset := vflag.NewFlagSet("dig", vflag.ContinueOnError)
	fset.Stdout = params.Stdout
	fset.Stderr = params.Stderr

	var httpsPathFlag string
	fset.AddLongFlagDig(&vflag.LongFlag{
		Description:  []string{"Enable DNS-over-HTTPS with optional `URL_PATH`."},
		ArgumentName: "[=STRING]",
		DefaultValue: "/dns-query",
		Name:         "https",
		MakeOption:   vflag.LongFlagMakeOptionWithOptionalValue,
		Prefix:       "--",
		Value:        vflag.NewValueString(&httpsPathFlag),
	})

	var shortFlag bool
	fset.AddLongFlagDig(vflag.NewLongFlagBool(
		vflag.NewValueBool(&shortFlag), "short", "Write terse output.",
	))

	fset.AutoHelp('h', "help", "Print this help message and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 4

	if err := fset.Parse(params.Argv[1:]); err != nil {
		if errors.Is(err, vflag.ErrHelp) {
			fset.PrintUsageString(params.Stdout)
			return nil
		}
		fset.PrintUsageError(params.Stderr, err)
		return err
	}

	// Parse positional args: [@server] [type] [class] domain
	server, qtype, domain, err := parseDigArgs(fset.Args())
	if err != nil {
		fmt.Fprintf(params.Stderr, "%s\n", err.Error())
		return err
	}

	// Resolve the server name to an IP address.
	serverAddr, err := r.resolveServer(ctx, server)
	if err != nil {
		fmt.Fprintf(params.Stderr, "%s\n", err.Error())
		return err
	}

	// Build the query.
	query := dnscodec.NewQuery(domain, qtype)

	// Exchange via the appropriate transport.
	var resp *dnscodec.Response
	if httpsPathFlag != "" {
		resp, err = r.digHTTPS(ctx, server, httpsPathFlag, query)
	} else {
		resp, err = r.digUDP(ctx, serverAddr, query)
	}
	if err != nil {
		fmt.Fprintf(params.Stderr, "%s\n", err.Error())
		return err
	}

	// Print the response.
	if shortFlag {
		for _, rr := range resp.Response.Answer {
			fmt.Fprintf(params.Stdout, "%s\n", shortAnswer(rr))
		}
	} else {
		fmt.Fprintf(params.Stdout, "%s\n", resp.Response.String())
	}
	return nil
}

// digUDP performs a DNS query over UDP.
func (r *Runner) digUDP(ctx context.Context, serverAddr netip.AddrPort, query *dnscodec.Query) (*dnscodec.Response, error) {
	txp := minest.NewDNSOverUDPTransport(r.sim, serverAddr)
	return txp.Exchange(ctx, query)
}

// digHTTPS performs a DNS query over HTTPS.
func (r *Runner) digHTTPS(ctx context.Context, server, path string, query *dnscodec.Query) (*dnscodec.Response, error) {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext:       r.sim.DialContext,
			ForceAttemptHTTP2: true,
			TLSClientConfig: &tls.Config{
				NextProtos: []string{"h2", "http/1.1"},
				RootCAs:    r.sim.CertPool(),
			},
		},
	}
	serverURL := &url.URL{Scheme: "https", Host: server, Path: path}
	txp := dnsoverhttps.NewTransport(client, serverURL.String())
	return txp.Exchange(ctx, query)
}

// resolveServer resolves a server name to a netip.AddrPort.
// If the server is already an IP address, it is used directly.
func (r *Runner) resolveServer(ctx context.Context, server string) (netip.AddrPort, error) {
	// Try parsing as IP literal first.
	if addr, err := netip.ParseAddr(server); err == nil {
		return netip.AddrPortFrom(addr, 53), nil
	}

	// Resolve the server name.
	addrs, err := r.sim.LookupHost(ctx, server)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("resolving %s: %w", server, err)
	}
	if len(addrs) < 1 {
		return netip.AddrPort{}, fmt.Errorf("resolving %s: no addresses", server)
	}
	addr, err := netip.ParseAddr(addrs[0])
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("resolving %s: %w", server, err)
	}
	return netip.AddrPortFrom(addr, 53), nil
}

// parseDigArgs parses the positional arguments for dig.
//
// Format: [@server] [type] domain
func parseDigArgs(args []string) (server string, qtype uint16, domain string, err error) {
	// Defaults.
	server = ""
	qtype = 0

	// Parse the arguments.
	var rest []string
	for _, arg := range args {
		switch {
		case server == "" && strings.HasPrefix(arg, "@"):
			server = arg[1:]

		case qtype == 0 && dns.StringToType[arg] != 0:
			qtype = dns.StringToType[arg]

		case domain == "" && arg != "IN":
			domain = arg

		case arg == "IN":
			// nothing

		default:
			rest = append(rest, arg)
		}
	}

	// Assign defaults.
	if server == "" {
		server = "8.8.8.8"
	}
	if qtype == 0 {
		qtype = dns.TypeA
	}

	// Make sure we consumed all arguments.
	if len(rest) > 0 {
		return "", 0, "", fmt.Errorf("excess command line arguments: %v", rest)
	}

	return server, qtype, domain, nil
}

// shortAnswer extracts the short answer from a DNS RR.
func shortAnswer(rr dns.RR) string {
	switch v := rr.(type) {
	case *dns.A:
		return v.A.String()
	case *dns.AAAA:
		return v.AAAA.String()
	case *dns.CNAME:
		return v.Target
	default:
		return rr.String()
	}
}
