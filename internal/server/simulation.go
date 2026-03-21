// SPDX-License-Identifier: GPL-3.0-or-later

package server

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/bassosimone/iss"
)

// Simulation wraps an [*iss.Simulation] to dispatch commands.
//
// Use [NewSimulation] to construct.
type Simulation struct {
	// sim is the underlying [*iss.Simulation].
	sim *iss.Simulation
}

// NewSimulation creates a [*Simulation] from a pre-configured
// [*iss.Simulation]. The caller is responsible for constructing
// the router, DPI engine, and any other components.
func NewSimulation(sim *iss.Simulation) *Simulation {
	return &Simulation{sim: sim}
}

// Wait waits for the simulation to shut down. Cancel the context
// passed to [iss.MustNewSimulation] before calling this method.
func (s *Simulation) Wait() {
	s.sim.Wait()
}

// RunCommandParams contains the parameters for [*Simulation.RunCommand].
type RunCommandParams struct {
	// Argv is the command line to execute.
	Argv []string

	// Stdout is the standard output writer.
	Stdout io.Writer

	// Stderr is the standard error writer.
	Stderr io.Writer
}

// ErrRunCommandNoCommand indicates that [RunCommandParams.Argv] is empty.
var ErrRunCommandNoCommand = errors.New("server: run: no command specified")

// ErrRunCommandUnknownCommand indicates that the command is not recognized.
var ErrRunCommandUnknownCommand = errors.New("server: run: unknown command")

// RunCommand runs a command within the simulation.
func (s *Simulation) RunCommand(ctx context.Context, params *RunCommandParams) error {
	if len(params.Argv) < 1 {
		return ErrRunCommandNoCommand
	}

	switch params.Argv[0] {
	case "curl":
		return s.runCurl(ctx, params)

	case "dig":
		return s.runDig(ctx, params)

	case "host":
		return s.runHost(ctx, params)

	default:
		return fmt.Errorf("%w: %s", ErrRunCommandUnknownCommand, params.Argv[0])
	}
}
