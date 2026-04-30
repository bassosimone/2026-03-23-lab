// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"log"
	"os"

	"github.com/bassosimone/deferexit"
	"github.com/bassosimone/runtimex"
	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

func main() {
	// Route runtimex.LogFatalOnError* and explicit exits through a typed
	// panic so deferred cleanup (e.g., removing data/url.txt) actually
	// runs. Recover here turns the panic back into a real os.Exit.
	defer deferexit.Recover(os.Exit)
	runtimex.LogFatalFunc = func(v ...any) {
		log.Print(v...)
		deferexit.Panic(1)
	}

	disp := vclip.NewDispatcherCommand("2026-03-23-lab", vflag.ExitOnError)
	disp.AddDescription("Internet censorship simulation.")

	disp.AddCommand("browser", vclip.CommandFunc(browserMain), "Open the lab in the default browser.")
	disp.AddCommand("dpi", vclip.CommandFunc(dpiMain), "Set DPI rules from a JSON file.")
	disp.AddCommand("pktlog", pktlogCommand(), "Interact with the packet log.")
	disp.AddCommand("run", vclip.CommandFunc(runMain), "Run a command inside the simulation.")
	disp.AddCommand("serve", vclip.CommandFunc(serveMain), "Start the simulation HTTP API server.")

	vclip.Main(context.Background(), disp, os.Args[1:])
}
