// SPDX-License-Identifier: GPL-3.0-or-later

package command

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/bassosimone/iss"
)

// Runner dispatches and runs commands using the client-side
// gVisor network stack of an [*iss.Simulation].
//
// Use [NewRunner] to construct.
type Runner struct {
	// sim is the underlying [*iss.Simulation].
	sim *iss.Simulation
}

// NewRunner creates a new [*Runner] for the given simulation.
func NewRunner(sim *iss.Simulation) *Runner {
	return &Runner{sim: sim}
}

// Params contains the parameters for [*Runner.Run].
type Params struct {
	// Argv is the command line to execute.
	Argv []string

	// Stdout is the standard output writer.
	Stdout io.Writer

	// Stderr is the standard error writer.
	Stderr io.Writer
}

// ErrNoCommand indicates that [Params.Argv] is empty.
var ErrNoCommand = errors.New("command: no command specified")

// ErrUnknownCommand indicates that the command is not recognized.
var ErrUnknownCommand = errors.New("command: unknown command")

// Run runs a command within the simulation.
func (r *Runner) Run(ctx context.Context, params *Params) error {
	if len(params.Argv) < 1 {
		return ErrNoCommand
	}

	switch params.Argv[0] {
	case "curl":
		return r.runCurl(ctx, params)

	case "dig":
		return r.runDig(ctx, params)

	case "host":
		return r.runHost(ctx, params)

	default:
		return fmt.Errorf("%w: %s", ErrUnknownCommand, params.Argv[0])
	}
}
