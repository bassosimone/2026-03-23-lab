// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/bassosimone/vflag"
)

// runHost implements the "host" command: resolves a domain
// name and prints the resulting addresses.
func (s *Simulation) runHost(ctx context.Context, params *RunCommandParams) error {
	// Parse flags.
	fset := vflag.NewFlagSet("host", vflag.ContinueOnError)
	fset.Stdout = params.Stdout
	fset.Stderr = params.Stderr
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
	domain := fset.Args()[0]

	addrs, err := s.sim.LookupHost(ctx, domain)
	if err != nil {
		fmt.Fprintf(params.Stderr, "%s\n", err.Error())
		return err
	}

	for _, addr := range addrs {
		fmt.Fprintf(params.Stdout, "%s has address %s\n", domain, addr)
	}
	return nil
}
