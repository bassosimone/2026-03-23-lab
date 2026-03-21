// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func dpiMain(ctx context.Context, args []string) error {
	// Parse flags.
	fset := vflag.NewFlagSet("2026-03-23-lab dpi", vflag.ExitOnError)
	datadir := "testdata"
	fset.StringVar(&datadir, 'd', "datadir", "The `DIR` containing the server URL file.")
	fset.AutoHelp('h', "help", "Print this help message and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = 1
	runtimex.PanicOnError0(fset.Parse(args))

	// Read the server base URL.
	baseURL := readBaseURL(datadir)

	// Read the JSON rules file.
	body := runtimex.LogFatalOnError1(os.ReadFile(fset.Args()[0]))

	// POST to the /api/dpi endpoint.
	reqURL := runtimex.LogFatalOnError1(url.JoinPath(baseURL, "api/dpi"))
	req := runtimex.LogFatalOnError1(http.NewRequestWithContext(
		ctx, http.MethodPost, reqURL, bytes.NewReader(body),
	))
	req.Header.Set("Content-Type", "application/json")
	resp := runtimex.LogFatalOnError1(http.DefaultClient.Do(req))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		runtimex.LogFatalOnError0(fmt.Errorf("server returned %s", resp.Status))
	}
	return nil
}
