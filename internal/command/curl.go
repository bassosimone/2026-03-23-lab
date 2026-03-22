// SPDX-License-Identifier: GPL-3.0-or-later

package command

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"

	"github.com/bassosimone/vflag"
)

// runCurl implements the "curl" command: fetches a URL using
// the simulated network and prints the response body.
func (r *Runner) runCurl(ctx context.Context, params *Params) error {
	// Parse flags.
	fset := vflag.NewFlagSet("curl", vflag.ContinueOnError)
	fset.Stdout = params.Stdout
	fset.Stderr = params.Stderr
	var (
		maxTime  float64
		output   string
		progress bool
		resolve  string
		verbose  bool
	)
	fset.Float64Var(&maxTime, 'm', "max-time", "Maximum time in `SECONDS` for the whole operation.")
	fset.StringVar(&output, 'o', "output", "Write body to `FILE` (use /dev/null to discard).")
	fset.BoolVar(&progress, '#', "progress-bar", "Show a progress bar.")
	fset.BoolVar(&verbose, 'v', "verbose", "Print request and response headers.")
	fset.StringVar(&resolve, 0, "resolve", "Override DNS as `HOST:PORT:ADDR`.")
	fset.AutoHelp('h', "help", "Print this help message and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	if err := fset.Parse(params.Argv[1:]); err != nil {
		if errors.Is(err, vflag.ErrHelp) {
			fset.PrintUsageString(params.Stdout)
			return nil
		}
		fset.PrintUsageError(params.Stderr, err)
		return err
	}
	rawURL := fset.Args()[0]

	// Apply --max-time timeout if provided.
	if maxTime > 0 {
		timeout := time.Duration(maxTime * float64(time.Second))
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Parse --resolve if provided.
	var rhost, rport, raddr string
	if resolve != "" {
		var err error
		rhost, rport, raddr, err = parseResolve(resolve)
		if err != nil {
			fmt.Fprintf(params.Stderr, "%s\n", err.Error())
			return err
		}
	}

	// Build the dial function with optional --resolve override
	// and optional verbose connection logging.
	dialFunc := func(ctx context.Context, network, address string) (net.Conn, error) {
		if resolve != "" {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if host == rhost && port == rport {
				address = net.JoinHostPort(raddr, port)
			}
		}
		if verbose {
			fmt.Fprintf(params.Stderr, "* Connecting to %s...\n", address)
		}
		conn, err := r.sim.DialContext(ctx, network, address)
		if err != nil {
			if verbose {
				fmt.Fprintf(params.Stderr, "* Connect to %s... %s\n", address, err)
			}
			return nil, err
		}
		if verbose {
			fmt.Fprintf(params.Stderr, "* Connected to %s... ok\n", address)
		}
		return conn, nil
	}

	// Build the HTTP client using the simulated network.
	client := &http.Client{
		Transport: &http.Transport{
			DialContext:       dialFunc,
			ForceAttemptHTTP2: true,
			TLSClientConfig: &tls.Config{
				NextProtos: []string{"h2", "http/1.1"},
				RootCAs:    r.sim.CertPool(),
			},
		},
	}

	// Build the request.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		fmt.Fprintf(params.Stderr, "%s\n", err.Error())
		return err
	}

	// If verbose, trace TLS and HTTP events.
	if verbose {
		trace := &httptrace.ClientTrace{
			TLSHandshakeStart: func() {
				fmt.Fprintf(params.Stderr, "* TLS handshake...\n")
			},
			TLSHandshakeDone: func(state tls.ConnectionState, err error) {
				if err != nil {
					fmt.Fprintf(params.Stderr, "* TLS handshake... %s\n", err)
				} else {
					fmt.Fprintf(params.Stderr, "* TLS handshake... ok (%s)\n", state.NegotiatedProtocol)
				}
			},
			GotConn: func(info httptrace.GotConnInfo) {
				fmt.Fprintf(params.Stderr, "> GET %s\n", req.URL.RequestURI())
				fmt.Fprintf(params.Stderr, "> Host: %s\n>\n", req.URL.Host)
			},
		}
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	}

	// Perform the request.
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(params.Stderr, "%s\n", err.Error())
		return err
	}
	defer resp.Body.Close()

	// Print verbose response headers.
	if verbose {
		fmt.Fprintf(params.Stderr, "< %s\n", resp.Status)
		for key, values := range resp.Header {
			for _, value := range values {
				fmt.Fprintf(params.Stderr, "< %s: %s\n", key, value)
			}
		}
		fmt.Fprintf(params.Stderr, "<\n")
	}

	// Determine the output destination.
	var dst io.Writer
	if output != "" {
		dst = io.Discard
	} else {
		dst = params.Stdout
	}

	// Copy the response body, optionally showing progress.
	if progress {
		err = copyWithProgress(dst, resp.Body, resp.ContentLength, params.Stderr)
	} else {
		_, err = io.Copy(dst, resp.Body)
	}
	if err != nil {
		fmt.Fprintf(params.Stderr, "%s\n", err.Error())
		return err
	}
	return nil
}

// copyWithProgress copies src to dst while printing a progress bar
// to w. If total is <= 0, only speed is shown (no percentage).
func copyWithProgress(dst io.Writer, src io.Reader, total int64, w io.Writer) error {
	buf := make([]byte, 32*1024)
	var copied int64
	start := time.Now()
	lastUpdate := start

	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return err
			}
			copied += int64(n)

			// Update the progress bar at most every 200ms.
			if now := time.Now(); now.Sub(lastUpdate) >= 200*time.Millisecond {
				lastUpdate = now
				printProgress(w, copied, total, now.Sub(start))
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				printProgress(w, copied, total, time.Since(start))
				fmt.Fprintf(w, "\n")
				return nil
			}
			return readErr
		}
	}
}

// printProgress prints a single progress line with \r.
func printProgress(w io.Writer, copied, total int64, elapsed time.Duration) {
	speed := float64(copied) / elapsed.Seconds()
	speedStr := formatSpeed(speed)

	if total > 0 {
		pct := float64(copied) / float64(total) * 100
		barWidth := 25
		filled := int(pct / 100 * float64(barWidth))
		bar := strings.Repeat("=", filled) + strings.Repeat(" ", barWidth-filled)
		fmt.Fprintf(w, "\r%3.0f%% [%s] %s  ", pct, bar, speedStr)
	} else {
		fmt.Fprintf(w, "\r%s received at %s  ", formatBytes(copied), speedStr)
	}
}

// formatSpeed formats a speed in bytes/sec to a human-readable string.
func formatSpeed(bps float64) string {
	switch {
	case bps >= 1<<20:
		return fmt.Sprintf("%.1f MB/s", bps/float64(1<<20))
	case bps >= 1<<10:
		return fmt.Sprintf("%.1f KB/s", bps/float64(1<<10))
	default:
		return fmt.Sprintf("%.0f B/s", bps)
	}
}

// formatBytes formats a byte count to a human-readable string.
func formatBytes(b int64) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// parseResolve parses a --resolve value in the form "host:port:addr".
func parseResolve(value string) (host, port, addr string, err error) {
	parts := strings.SplitN(value, ":", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("invalid --resolve value: %q (expected host:port:addr)", value)
	}
	return parts[0], parts[1], parts[2], nil
}
