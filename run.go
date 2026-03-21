// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func runMain(ctx context.Context, args []string) error {
	// Parse flags.
	fset := vflag.NewFlagSet("2026-03-23-lab run", vflag.ExitOnError)
	datadir := "testdata"
	fset.StringVar(&datadir, 'd', "datadir", "The `DIR` containing the server URL file.")
	fset.AutoHelp('h', "help", "Print this help message and exit.")
	fset.MinPositionalArgs = 1
	fset.MaxPositionalArgs = math.MaxInt
	fset.DisablePermute = true
	runtimex.PanicOnError0(fset.Parse(args))

	// Read the server base URL.
	urlData := runtimex.LogFatalOnError1(os.ReadFile(filepath.Join(datadir, "url.txt")))
	baseURL := strings.TrimSpace(string(urlData))

	// Build the JSON request body.
	body := runtimex.LogFatalOnError1(json.Marshal(struct {
		Argv []string `json:"argv"`
	}{Argv: fset.Args()}))

	// POST to the /api/run endpoint.
	req := runtimex.LogFatalOnError1(http.NewRequestWithContext(
		ctx, http.MethodPost, baseURL+"api/run", bytes.NewReader(body),
	))
	req.Header.Set("Content-Type", "application/json")
	resp := runtimex.LogFatalOnError1(http.DefaultClient.Do(req))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		runtimex.LogFatalOnError0(fmt.Errorf("server returned %s", resp.Status))
	}

	// Parse the SSE event stream.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	var eventType string
	exitcode := 1
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			eventType = strings.TrimPrefix(line, "event: ")

		case strings.HasPrefix(line, "data: "):
			data := strings.TrimPrefix(line, "data: ")
			switch eventType {
			case "stdout":
				decoded := runtimex.LogFatalOnError1(base64.StdEncoding.DecodeString(data))
				os.Stdout.Write(decoded)
			case "stderr":
				decoded := runtimex.LogFatalOnError1(base64.StdEncoding.DecodeString(data))
				os.Stderr.Write(decoded)
			case "exitcode":
				exitcode = runtimex.LogFatalOnError1(strconv.Atoi(data))
			}

		case line == "":
			eventType = ""
		}
	}
	runtimex.LogFatalOnError0(scanner.Err())

	if exitcode != 0 {
		os.Exit(exitcode)
	}
	return nil
}
