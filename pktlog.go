// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

// pktlogCommand returns a [vclip.Command] dispatcher for the
// "pktlog" subcommand with "clear" and "get" subcommands.
func pktlogCommand() *vclip.DispatcherCommand {
	disp := vclip.NewDispatcherCommand("2026-03-23-lab pktlog", vflag.ExitOnError)
	disp.AddDescription("Interact with the packet log.")
	disp.AddCommand("clear", vclip.CommandFunc(pktlogClearMain), "Clear the packet log.")
	disp.AddCommand("get", vclip.CommandFunc(pktlogGetMain), "Download the packet log.")
	return disp
}

// pktlogClearMain implements "pktlog clear".
func pktlogClearMain(ctx context.Context, args []string) error {
	// Parse flags.
	fset := vflag.NewFlagSet("2026-03-23-lab pktlog clear", vflag.ExitOnError)
	datadir := "data"
	fset.StringVar(&datadir, 'd', "datadir", "The `DIR` containing the server URL file.")
	fset.AutoHelp('h', "help", "Print this help message and exit.")
	runtimex.PanicOnError0(fset.Parse(args))

	// Read the server base URL.
	baseURL := readBaseURL(datadir)

	// DELETE /api/pktlog.
	req := runtimex.LogFatalOnError1(http.NewRequestWithContext(
		ctx, http.MethodDelete, runtimex.LogFatalOnError1(url.JoinPath(baseURL, "api/pktlog")), nil,
	))
	resp := runtimex.LogFatalOnError1(http.DefaultClient.Do(req))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		runtimex.LogFatalOnError0(fmt.Errorf("server returned %s", resp.Status))
	}
	return nil
}

// pktlogGetMain implements "pktlog get".
func pktlogGetMain(ctx context.Context, args []string) error {
	// Parse flags.
	fset := vflag.NewFlagSet("2026-03-23-lab pktlog get", vflag.ExitOnError)
	datadir := "data"
	format := "pcap"
	var addr string
	var output string
	fset.StringVar(&datadir, 'd', "datadir", "The `DIR` containing the server URL file.")
	fset.StringVar(&format, 0, "format", "Output `FORMAT` (json, pcap).")
	fset.StringVar(&addr, 0, "addr", "Filter by `IP` address perspective.")
	fset.StringVar(&output, 'o', "output", "Write to `FILE` instead of stdout.")
	fset.AutoHelp('h', "help", "Print this help message and exit.")
	runtimex.PanicOnError0(fset.Parse(args))

	// Validate format-specific constraints.
	switch format {
	case "json":
		// addr is optional for json format
	case "pcap":
		if addr == "" {
			runtimex.LogFatalOnError0(fmt.Errorf("pcap format requires --addr"))
		}
		if output == "" {
			runtimex.LogFatalOnError0(fmt.Errorf("pcap format requires -o/--output"))
		}
	default:
		runtimex.LogFatalOnError0(fmt.Errorf("unsupported format: %q", format))
	}

	// Read the server base URL.
	baseURL := readBaseURL(datadir)

	// Build the query URL.
	reqURL := runtimex.LogFatalOnError1(url.JoinPath(baseURL, "api/pktlog"))
	query := url.Values{}
	query.Set("format", format)
	query.Set("addr", addr)
	reqURL = reqURL + "?" + query.Encode()

	// GET /api/pktlog.
	req := runtimex.LogFatalOnError1(http.NewRequestWithContext(
		ctx, http.MethodGet, reqURL, nil,
	))
	resp := runtimex.LogFatalOnError1(http.DefaultClient.Do(req))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		runtimex.LogFatalOnError0(fmt.Errorf("server returned %s", resp.Status))
	}

	// Write the response body to the output destination.
	var dst io.Writer
	if output != "" {
		fp := runtimex.LogFatalOnError1(os.Create(output))
		defer fp.Close()
		dst = fp
	} else {
		dst = os.Stdout
	}
	runtimex.LogFatalOnError1(io.Copy(dst, resp.Body))
	return nil
}

// readBaseURL reads the server base URL from the datadir.
func readBaseURL(datadir string) string {
	data := runtimex.LogFatalOnError1(os.ReadFile(filepath.Join(datadir, "url.txt")))
	return strings.TrimSpace(string(data))
}
