// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"os"

	"github.com/bassosimone/vclip"
	"github.com/bassosimone/vflag"
)

func main() {
	disp := vclip.NewDispatcherCommand("2026-03-23-lab", vflag.ExitOnError)
	disp.AddDescription("Internet censorship simulation.")

	disp.AddCommand("dpi", vclip.CommandFunc(dpiMain), "Set DPI rules from a JSON file.")
	disp.AddCommand("pktlog", pktlogCommand(), "Interact with the packet log.")
	disp.AddCommand("run", vclip.CommandFunc(runMain), "Run a command inside the simulation.")
	disp.AddCommand("serve", vclip.CommandFunc(serveMain), "Start the simulation HTTP API server.")

	vclip.Main(context.Background(), disp, os.Args[1:])
}
