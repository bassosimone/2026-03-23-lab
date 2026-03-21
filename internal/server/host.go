// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"fmt"
)

// ErrHostUsage indicates invalid usage of the host command.
var ErrHostUsage = errors.New("usage: host <domain>")

// runHost implements the "host" command: resolves a domain
// name and prints the resulting addresses.
func (s *Simulation) runHost(ctx context.Context, params *RunCommandParams) error {
	if len(params.Argv) != 2 {
		err := ErrHostUsage
		fmt.Fprintf(params.Stderr, "%s\n", err.Error())
		return err
	}
	domain := params.Argv[1]

	addrs, err := s.sim.LookupHost(ctx, domain)
	if err != nil {
		return err
	}

	for _, addr := range addrs {
		fmt.Fprintf(params.Stdout, "%s has address %s\n", domain, addr)
	}
	return nil
}
