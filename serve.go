// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bassosimone/2026-03-23-lab/internal/command"
	"github.com/bassosimone/2026-03-23-lab/internal/httpapi"
	"github.com/bassosimone/2026-03-23-lab/internal/pktlog"
	"github.com/bassosimone/2026-03-23-lab/internal/vis"
	"github.com/bassosimone/iss"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vflag"
)

func serveMain(ctx context.Context, args []string) error {
	// Parse flags.
	fset := vflag.NewFlagSet("2026-03-23-lab serve", vflag.ExitOnError)
	addr := "127.0.0.1:0"
	datadir := "data"
	fset.StringVar(&addr, 0, "addr", "The `ADDRESS` to listen on.")
	fset.StringVar(&datadir, 'd', "datadir", "The `DIR` for cached PKI certificates.")
	fset.AutoHelp('h', "help", "Print this help message and exit.")
	runtimex.PanicOnError0(fset.Parse(args)) // We are using vflag.ExitOnError

	// Create the DPI engine, packet logger, and router.
	dpi := vis.NewDPIEngine()
	logger := pktlog.NewLogger(10000)
	router := vis.NewRouter(
		vis.RouterOptionDelay(10*time.Millisecond),
		vis.RouterOptionDPI(dpi),
		vis.RouterOptionHook(logger.Hook),
	)

	// Create and start the simulation.
	sim := iss.MustNewSimulation(ctx, datadir, iss.ScenarioV4(), router)
	defer sim.Wait()
	runner := command.NewRunner(sim)

	// Create the HTTP API handler and register routes.
	mux := http.NewServeMux()
	handler := httpapi.NewHandler(runner, dpi, logger)
	handler.Register(mux)

	// Start listening and save the base URL for clients to discover.
	listener := runtimex.LogFatalOnError1(net.Listen("tcp", addr))
	defer listener.Close()
	baseURL := fmt.Sprintf("http://%s/", listener.Addr())
	fmt.Fprintf(os.Stderr, "listening on %s\n", baseURL)
	urlFile := filepath.Join(datadir, "url.txt")
	runtimex.LogFatalOnError0(os.WriteFile(urlFile, []byte(baseURL), 0644))

	// Serve HTTP until the context is canceled.
	srv := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	err := srv.Serve(listener)

	// Make sure we always remove the URL file.
	os.Remove(urlFile)

	// Decide whether there was a real error.
	if err == http.ErrServerClosed {
		err = nil
	}
	runtimex.LogFatalOnError0(err)
	return nil
}
